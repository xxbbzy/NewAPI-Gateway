package service

import (
	"NewAPI-Gateway/common"
	"archive/zip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BackupManifest struct {
	FormatVersion string `json:"format_version"`
	AppVersion    string `json:"app_version"`
	CreatedAt     int64  `json:"created_at"`
	Driver        string `json:"driver"`
	PayloadFile   string `json:"payload_file"`
	PayloadFormat string `json:"payload_format"`
	PayloadSHA256 string `json:"payload_sha256"`
	TriggerType   string `json:"trigger_type"`
}

type BackupArtifact struct {
	LocalPath    string
	Manifest     BackupManifest
	PayloadSize  int64
	Encrypted    bool
	RelativeName string
}

func buildBackupArtifact(workDir string, snapshot *SnapshotResult, driver string, trigger string, cfg BackupConfig) (*BackupArtifact, error) {
	payloadBytes, err := os.ReadFile(snapshot.PayloadPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payloadBytes)
	payloadBase := filepath.Base(snapshot.PayloadPath)
	manifest := BackupManifest{
		FormatVersion: "v1",
		AppVersion:    common.Version,
		CreatedAt:     time.Now().Unix(),
		Driver:        driver,
		PayloadFile:   payloadBase,
		PayloadFormat: snapshot.Format,
		PayloadSHA256: hex.EncodeToString(sum[:]),
		TriggerType:   trigger,
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}

	rawPath := filepath.Join(workDir, "backup-package.zip")
	if err := writeZipWithManifest(rawPath, payloadBase, payloadBytes, manifestBytes); err != nil {
		return nil, err
	}
	finalPath := rawPath
	encrypted := false
	if cfg.EncryptEnabled {
		if strings.TrimSpace(cfg.EncryptPassphrase) == "" {
			return nil, fmt.Errorf("backup encryption is enabled but passphrase is empty")
		}
		encryptedPath := rawPath + ".enc"
		if err := encryptFileAESGCM(rawPath, encryptedPath, cfg.EncryptPassphrase); err != nil {
			return nil, err
		}
		_ = os.Remove(rawPath)
		finalPath = encryptedPath
		encrypted = true
	}
	st, statErr := os.Stat(finalPath)
	if statErr != nil {
		return nil, statErr
	}
	relName := fmt.Sprintf("%s-%s-%d.zip", driver, trigger, time.Now().Unix())
	if encrypted {
		relName += ".enc"
	}
	return &BackupArtifact{
		LocalPath:    finalPath,
		Manifest:     manifest,
		PayloadSize:  st.Size(),
		Encrypted:    encrypted,
		RelativeName: relName,
	}, nil
}

func writeZipWithManifest(targetPath string, payloadName string, payload []byte, manifest []byte) error {
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()
	zipWriter := zip.NewWriter(file)
	payloadWriter, err := zipWriter.Create(payloadName)
	if err != nil {
		_ = zipWriter.Close()
		return err
	}
	if _, err := payloadWriter.Write(payload); err != nil {
		_ = zipWriter.Close()
		return err
	}
	manifestWriter, err := zipWriter.Create("manifest.json")
	if err != nil {
		_ = zipWriter.Close()
		return err
	}
	if _, err := manifestWriter.Write(manifest); err != nil {
		_ = zipWriter.Close()
		return err
	}
	return zipWriter.Close()
}

func deriveEncryptionKey(passphrase string) [32]byte {
	return sha256.Sum256([]byte(passphrase))
}

func encryptFileAESGCM(srcPath string, dstPath string, passphrase string) error {
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	key := deriveEncryptionKey(passphrase)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	result := append(nonce, ciphertext...)
	return os.WriteFile(dstPath, result, 0o600)
}

func decryptFileAESGCM(srcPath string, dstPath string, passphrase string) error {
	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	key := deriveEncryptionKey(passphrase)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	body := ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return fmt.Errorf("decrypt failed: %w", err)
	}
	return os.WriteFile(dstPath, plaintext, 0o600)
}

func unpackArtifactAndManifest(artifactPath string, cfg BackupConfig) (string, *BackupManifest, error) {
	zipPath := artifactPath
	cleanupPlaintext := false
	if strings.HasSuffix(strings.ToLower(artifactPath), ".enc") {
		if strings.TrimSpace(cfg.EncryptPassphrase) == "" {
			return "", nil, fmt.Errorf("artifact is encrypted but passphrase is empty")
		}
		zipPath = strings.TrimSuffix(artifactPath, ".enc")
		if err := decryptFileAESGCM(artifactPath, zipPath, cfg.EncryptPassphrase); err != nil {
			return "", nil, err
		}
		cleanupPlaintext = true
	}
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		if cleanupPlaintext {
			_ = os.Remove(zipPath)
		}
		return "", nil, err
	}
	defer reader.Close()
	var manifest BackupManifest
	var payloadPath string
	baseDir := filepath.Dir(zipPath)
	for _, f := range reader.File {
		rc, openErr := f.Open()
		if openErr != nil {
			if cleanupPlaintext {
				_ = os.Remove(zipPath)
			}
			return "", nil, openErr
		}
		data, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			if cleanupPlaintext {
				_ = os.Remove(zipPath)
			}
			return "", nil, readErr
		}
		if f.Name == "manifest.json" {
			if err := json.Unmarshal(data, &manifest); err != nil {
				if cleanupPlaintext {
					_ = os.Remove(zipPath)
				}
				return "", nil, err
			}
			continue
		}
		payloadPath = filepath.Join(baseDir, filepath.Base(f.Name))
		if err := os.WriteFile(payloadPath, data, 0o600); err != nil {
			if cleanupPlaintext {
				_ = os.Remove(zipPath)
			}
			return "", nil, err
		}
	}
	if cleanupPlaintext {
		_ = os.Remove(zipPath)
	}
	if payloadPath == "" {
		return "", nil, fmt.Errorf("payload file missing in artifact")
	}
	if strings.TrimSpace(manifest.PayloadSHA256) == "" {
		return "", nil, fmt.Errorf("manifest payload hash is missing")
	}
	return payloadPath, &manifest, nil
}

func verifyPayloadChecksum(path string, expectedSHA256 string) error {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(bytes)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(expectedSHA256)) {
		return fmt.Errorf("payload checksum mismatch")
	}
	return nil
}
