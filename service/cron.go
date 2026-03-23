package service

import (
	"NewAPI-Gateway/common"
	"NewAPI-Gateway/model"
	"time"
)

var syncTicker *time.Ticker
var checkinSchedulerTicker *time.Ticker
var stopCron chan bool
var syncProviderRunner = SyncProvider

func syncProviderBatch(providers []*model.Provider, label string) {
	for _, p := range providers {
		if err := syncProviderRunner(p); err != nil {
			common.SysLog(label + " failed for provider " + p.Name + ": " + err.Error())
		}
	}
}

func repairProvidersWithUnresolvedTokens() {
	providers, err := model.GetProvidersWithUnresolvedProviderTokens()
	if err != nil {
		common.SysLog("failed to get unresolved-token providers: " + err.Error())
		return
	}
	now := time.Now()
	targets := make([]*model.Provider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		if provider.Status != common.UserStatusEnabled {
			continue
		}
		if provider.CanParticipateInAutomatedUseAt(now) {
			continue
		}
		targets = append(targets, provider)
	}
	if len(targets) == 0 {
		return
	}
	common.SysLog("running targeted unresolved-token repair sync")
	syncProviderBatch(targets, "repair sync")
}

func runProviderSyncCycle() {
	providers, err := model.GetAutomatedSyncProviders()
	if err != nil {
		common.SysLog("failed to get automated-sync providers: " + err.Error())
	} else {
		syncProviderBatch(providers, "sync")
	}
	repairProvidersWithUnresolvedTokens()
}

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
				go runProviderSyncCycle()
			case <-checkinSchedulerTicker.C:
				go RunScheduledCheckinIfNeeded(time.Now())
				go EvaluateBackupScheduleIfNeeded(time.Now())
			case <-stopCron:
				syncTicker.Stop()
				checkinSchedulerTicker.Stop()
				return
			}
		}
	}()

	go RunScheduledCheckinIfNeeded(time.Now())
	go EvaluateBackupScheduleIfNeeded(time.Now())

	// Delayed initial sync to fix existing dirty sk_keys on upgrade
	go func() {
		time.Sleep(10 * time.Second)
		common.SysLog("running initial provider sync on startup")
		runProviderSyncCycle()
	}()

	common.SysLog("cron jobs started: sync every 5m, scheduled checkin evaluated every 1m, backup scheduler evaluated every 1m")
}

// StopCronJobs stops background tasks
func StopCronJobs() {
	if stopCron != nil {
		stopCron <- true
	}
}

func syncAllProviders() {
	runProviderSyncCycle()
}
