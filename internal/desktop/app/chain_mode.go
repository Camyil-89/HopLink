package desktop

import "strings"

const (
	ChainModeRelay   = "udp-relay"
	ChainModeCascade = "hy2-cascade"
)

func normalizeChainMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		return ChainModeRelay
	}
	return mode
}

func validChainMode(mode string) bool {
	mode = normalizeChainMode(mode)
	return mode == ChainModeRelay || mode == ChainModeCascade
}

func isCascadeMode(cfg AppConfig) bool {
	return normalizeChainMode(cfg.ChainMode) == ChainModeCascade
}