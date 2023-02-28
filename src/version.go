package authress

type BuildInfo struct {
    Version string
}

func GetBuildInfo() BuildInfo {
    return BuildInfo{
        Version: "0.0.0",
    }
}