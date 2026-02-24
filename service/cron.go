package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"time"
)

var syncTicker *time.Ticker
var checkinSchedulerTicker *time.Ticker
var stopCron chan bool

// StartCronJobs starts background sync and checkin tasks
func StartCronJobs() {
	stopCron = make(chan bool)

	// Sync every 5 minutes
	syncTicker = time.NewTicker(5 * time.Minute)
	// Check checkin schedule every minute
	checkinSchedulerTicker = time.NewTicker(1 * time.Minute)

	go func() {
		for {
			select {
			case <-syncTicker.C:
				go syncAllProviders()
			case <-checkinSchedulerTicker.C:
				go RunScheduledCheckinIfNeeded(time.Now())
			case <-stopCron:
				syncTicker.Stop()
				checkinSchedulerTicker.Stop()
				return
			}
		}
	}()

	go RunScheduledCheckinIfNeeded(time.Now())
	common.SysLog("cron jobs started: sync every 5m, scheduled checkin evaluated every 1m")
}

// StopCronJobs stops background tasks
func StopCronJobs() {
	if stopCron != nil {
		stopCron <- true
	}
}

func syncAllProviders() {
	providers, err := model.GetEnabledProviders()
	if err != nil {
		common.SysLog("failed to get enabled providers for sync: " + err.Error())
		return
	}
	for _, p := range providers {
		if err := SyncProvider(p); err != nil {
			common.SysLog("sync failed for provider " + p.Name + ": " + err.Error())
		}
	}
}
