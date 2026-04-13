package version

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// String возвращает полную версию
func String() string {
	return Version + " (" + Commit + ") built at " + BuildTime
}
