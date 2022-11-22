package version

var (
	// BuildDate is the timestamp of the build
	BuildDate string

	// CommitSHA is the commit SHA of the build
	CommitSHA string
)

func TruncatedCommitSha() string {
	return CommitSHA[:10]
}
