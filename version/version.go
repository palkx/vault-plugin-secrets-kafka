package version

var (
	version = "0.0.1"
	commit  = "snapshot"
	date    = "unknown"
)

func Version() string {
	return version
}

func Commit() string {
	return commit
}

func Date() string {
	return date
}

func Build() string {
	return version + "+" + commit
}
