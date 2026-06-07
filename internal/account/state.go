package account

import "tg-provider/internal/model"

var knownStatuses = map[string]struct{}{
	model.AccountStatusNew:           {},
	model.AccountStatusLoginRequired: {},
	model.AccountStatusSyncing:       {},
	model.AccountStatusOnline:        {},
	model.AccountStatusReconnecting:  {},
	model.AccountStatusFloodWait:     {},
	model.AccountStatusDisconnected:  {},
}

var allowedTransitions = map[string]map[string]struct{}{
	model.AccountStatusNew: {
		model.AccountStatusLoginRequired: {},
	},
	model.AccountStatusLoginRequired: {
		model.AccountStatusOnline:       {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusOnline: {
		model.AccountStatusSyncing:      {},
		model.AccountStatusReconnecting: {},
		model.AccountStatusFloodWait:    {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusSyncing: {
		model.AccountStatusOnline:       {},
		model.AccountStatusFloodWait:    {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusReconnecting: {
		model.AccountStatusOnline:       {},
		model.AccountStatusFloodWait:    {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusFloodWait: {
		model.AccountStatusReconnecting: {},
		model.AccountStatusDisconnected: {},
	},
	model.AccountStatusDisconnected: {
		model.AccountStatusReconnecting: {},
	},
}

func KnownStatus(status string) bool {
	_, ok := knownStatuses[status]
	return ok
}

func CanTransition(from string, to string) bool {
	if !KnownStatus(from) || !KnownStatus(to) {
		return false
	}
	if from == to {
		return true
	}
	targets, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}
