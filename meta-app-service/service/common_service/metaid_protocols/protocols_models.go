package metaid_protocols

import (
	"fmt"
	"strings"
)

/*
*

	{
		"title":"orders.exchange",
		"appName":"",
		"prompt":“”,
		"icon":"metafile://",
		"coverImg":"metafile://"
		"introImgs":["metafile://"],
		"intro":"introduction about this app",
		"runtime":"browser/android/ios/windows/macOS/Linux",
		"indexFile":"",
		"version":"",
		"contentType":"/protocols/metatree",
		"content":"pinid",
		"code":"metafile://pinid",
		"contentHash":"xxx",
		"metadata":"Arbitrary data"
		"disabled":"If left blank, the default value is false, which specifies whether this MetaApp should be displayed publicly"
	}

*
*/
type MetaApp struct {
	Title       string   `json:"title"`
	AppName     string   `json:"appName"`
	Prompt      string   `json:"prompt"`
	Icon        string   `json:"icon"`
	CoverImg    string   `json:"coverImg"`
	IntroImgs   []string `json:"introImgs"`
	Intro       string   `json:"intro"`
	Runtime     string   `json:"runtime"`
	IndexFile   string   `json:"indexFile"`
	Version     string   `json:"version"`
	ContentType string   `json:"contentType"`
	Content     string   `json:"content"`
	Code        string   `json:"code"`
	ContentHash string   `json:"contentHash"`
	Metadata    string   `json:"metadata"`
	Disabled    bool     `json:"disabled"`
}

// /file
const (
	MonitorMetaApp = "metaapp"
)

var (
	ProtocolList = []string{
		fmt.Sprintf("/protocols/%s", strings.ToLower(MonitorMetaApp)),
	}
)
