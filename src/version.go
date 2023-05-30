package authress

type BuildInfo struct {
    Version string
}

func GetBuildInfo() BuildInfo {
    return BuildInfo{
        // This should actually be set using: https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications
        Version: "0.0.0",
    }
}