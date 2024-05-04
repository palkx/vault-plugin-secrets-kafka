package version

var (
	version = "0.0.1"
	commit  = "snapshot"
	date    = "unknown"
	builtBy = "local"
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

func BuiltBy() string {
	return builtBy
}

func Build() string {
	return version + "+" + commit
}

func FullBuildInfo() string {
	return version + "+" + commit + "+" + date + "+" + builtBy
}
