package config

import (
	"os"
	"strconv"
	"strings"
)

func getInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return fallback
	}
	return parsed
}

func getCSV(key string, fallback string) []string {
	val := os.Getenv(key)
	if val == "" {
		val = fallback
	}
	if val == "" {
		return nil
	}
	var res []string
	for _, item := range strings.Split(val, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func getBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvFirst(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func getBoolMulti(keys []string, fallback bool) bool {
	val := getEnvFirst(keys...)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func getIntMulti(keys []string, fallback int) int {
	val := getEnvFirst(keys...)
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

// IsAllowedOrigin deliberately does not support a wildcard. Browser clients
// must be explicitly named; native clients send no Origin header.
func IsAllowedOrigin(origin string) bool {
	for _, allowed := range AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

var (
	MaxMembers           = getInt("JAM_MAX_MEMBERS", 5)
	StateHeartbeatMs     = getInt("JAM_STATE_HEARTBEAT_MS", 5000)
	PlayLeadMs           = getInt("JAM_PLAY_LEAD_MS", 350)
	DisconnectGraceMs    = int64(getInt("JAM_DISCONNECT_GRACE_MS", 45000))
	EmptyPartyTTLMs      = int64(getInt("JAM_EMPTY_PARTY_TTL_MS", 120000))
	PartyMaxAgeMs        = int64(getInt("JAM_PARTY_MAX_AGE_MS", 12*60*60*1000))
	ControlRatePerSecond = float64(getInt("JAM_CONTROL_RATE_PER_SECOND", 25))
	MaxUpcomingQueue     = getInt("JAM_MAX_UPCOMING_QUEUE", 25)
	MaxQueueLength       = getInt("JAM_MAX_QUEUE_LENGTH", 1+MaxUpcomingQueue)
	MaxParties           = getInt("JAM_MAX_PARTIES", 100)
	CreateRatePerMinute  = getInt("JAM_CREATE_RATE_PER_MINUTE", 2)
	RateLimitMaxEntries  = getInt("JAM_RATE_LIMIT_MAX_ENTRIES", 10000)
	RequestMaxBytes      = int64(getInt("JAM_REQUEST_MAX_BYTES", 16*1024))
	WebSocketMaxBytes    = int64(getInt("JAM_WEBSOCKET_MAX_BYTES", 16*1024))
	ConnectionIdleMs     = int64(getInt("JAM_CONNECTION_IDLE_MS", 15*60*1000))
	FrameRatePerSecond   = float64(getInt("JAM_FRAME_RATE_PER_SECOND", 30))
	AllowedOrigins       = getCSV("JAM_ALLOWED_ORIGINS", "")
	TrustProxy           = getBoolMulti([]string{"JAM_TRUST_PROXY", "TRUST_PROXY"}, true)
	Port                 = getIntMulti([]string{"PORT", "JAM_PORT"}, 8080)

	// Low-latency & connection limits
	TCPNoDelay               = getBool("JAM_TCP_NODELAY", true)
	WebSocketReadBufferSize  = getInt("JAM_WS_READ_BUFFER_SIZE", 2048)
	WebSocketWriteBufferSize = getInt("JAM_WS_WRITE_BUFFER_SIZE", 2048)
	MaxConcurrentSockets     = getInt("JAM_MAX_CONCURRENT_SOCKETS", 1000)
	MaxSocketsPerIP          = getInt("JAM_MAX_SOCKETS_PER_IP", 25)
)
