package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

type webDAVClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

type webDAVItem struct {
	Href         string
	ContentLen   int64
	LastModified time.Time
}

func newWebDAVClient(cfg BackupConfig) (*webDAVClient, error) {
	if strings.TrimSpace(cfg.WebDAVURL) == "" {
		return nil, fmt.Errorf("backup webdav url is empty")
	}
	parsed, err := url.Parse(cfg.WebDAVURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("webdav url must use http/https")
	}
	return &webDAVClient{
		baseURL:    strings.TrimRight(parsed.String(), "/"),
		username:   cfg.WebDAVUsername,
		password:   cfg.WebDAVPassword,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *webDAVClient) buildURL(p string) string {
	base := strings.TrimRight(c.baseURL, "/")
	clean := path.Clean("/" + strings.TrimSpace(p))
	if clean == "/" {
		return base + clean
	}
	return base + clean
}

func (c *webDAVClient) do(req *http.Request) (*http.Response, error) {
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	return c.httpClient.Do(req)
}

func (c *webDAVClient) ensureCollection(remoteDir string) error {
	target := c.buildURL(remoteDir)
	req, err := http.NewRequest("MKCOL", target, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("webdav mkcol failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c *webDAVClient) uploadFile(localPath string, remotePath string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, c.buildURL(remotePath), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("webdav upload failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c *webDAVClient) downloadFile(remotePath string, localPath string) error {
	req, err := http.NewRequest(http.MethodGet, c.buildURL(remotePath), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webdav download failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(localPath, content, 0o600)
}

func (c *webDAVClient) deleteFile(remotePath string) error {
	req, err := http.NewRequest(http.MethodDelete, c.buildURL(remotePath), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("webdav delete failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

type davMultiStatus struct {
	XMLName   xml.Name          `xml:"multistatus"`
	Responses []davPropResponse `xml:"response"`
}

type davPropResponse struct {
	Href     string        `xml:"href"`
	Propstat []davPropStat `xml:"propstat"`
}

type davPropStat struct {
	Prop davProp `xml:"prop"`
}

type davProp struct {
	GetContentLength string `xml:"getcontentlength"`
	GetLastModified  string `xml:"getlastmodified"`
	ResourceType     string `xml:"resourcetype"`
}

func (c *webDAVClient) listFiles(remoteDir string) ([]webDAVItem, error) {
	reqBody := `<?xml version="1.0" encoding="utf-8" ?><d:propfind xmlns:d="DAV:"><d:prop><d:getcontentlength/><d:getlastmodified/><d:resourcetype/></d:prop></d:propfind>`
	req, err := http.NewRequest("PROPFIND", c.buildURL(remoteDir), strings.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("webdav list failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result davMultiStatus
	if err := xml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	items := make([]webDAVItem, 0, len(result.Responses))
	for _, res := range result.Responses {
		href := strings.TrimSpace(res.Href)
		if href == "" || strings.HasSuffix(href, "/") {
			continue
		}
		item := webDAVItem{Href: href}
		if len(res.Propstat) > 0 {
			if size, sizeErr := strconv.ParseInt(strings.TrimSpace(res.Propstat[0].Prop.GetContentLength), 10, 64); sizeErr == nil {
				item.ContentLen = size
			}
			if lm := strings.TrimSpace(res.Propstat[0].Prop.GetLastModified); lm != "" {
				if t, tErr := time.Parse(time.RFC1123, lm); tErr == nil {
					item.LastModified = t
				}
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].LastModified.Before(items[j].LastModified)
	})
	return items, nil
}

func applyWebDAVRetention(client *webDAVClient, basePath string, maxDays int, maxFiles int) error {
	items, err := client.listFiles(basePath)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	now := time.Now()
	for _, item := range items {
		if maxDays > 0 && !item.LastModified.IsZero() {
			if now.Sub(item.LastModified) > time.Duration(maxDays)*24*time.Hour {
				if err := client.deleteFile(item.Href); err != nil {
					logBackupError("retention delete(old): " + err.Error())
				}
			}
		}
	}
	if maxFiles <= 0 {
		return nil
	}
	if len(items) <= maxFiles {
		return nil
	}
	toDelete := len(items) - maxFiles
	for idx := 0; idx < toDelete; idx++ {
		if err := client.deleteFile(items[idx].Href); err != nil {
			logBackupError("retention delete(count): " + err.Error())
		}
	}
	return nil
}
