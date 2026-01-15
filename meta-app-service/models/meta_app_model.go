package models

import "time"

// MetaApp MetaApp 协议数据模型
type MetaApp struct {
	// 基础信息
	FirstPinId string `json:"first_pin_id"` // 第一个 PIN ID
	PinID      string `json:"pin_id"`       // PIN ID (主键)
	TxID       string `json:"tx_id"`        // 交易 ID
	Vout       uint32 `json:"vout"`         // 输出索引
	Path       string `json:"path"`         // 路径
	Operation  string `json:"operation"`    // 操作类型: create/modify/revoke
	ParentPath string `json:"parent_path"`  // 父路径

	// MetaApp 协议字段
	Title       string   `json:"title"`        // 应用标题
	AppName     string   `json:"app_name"`     // 应用名称
	Prompt      string   `json:"prompt"`       // 提示信息
	Icon        string   `json:"icon"`         // 图标 (metafile://pinid)
	CoverImg    string   `json:"cover_img"`    // 封面图片 (metafile://pinid)
	IntroImgs   []string `json:"intro_imgs"`   // 介绍图片列表 (metafile://pinid)
	Intro       string   `json:"intro"`        // 应用介绍
	Runtime     string   `json:"runtime"`      // 运行环境: browser/android/ios/windows/macOS/Linux
	IndexFile   string   `json:"index_file"`   // 入口文件
	Version     string   `json:"version"`      // 版本号
	ContentType string   `json:"content_type"` // 内容类型: /protocols/metatree
	Content     string   `json:"content"`      // 内容 (pinid)
	Code        string   `json:"code"`         // 代码 (metafile://pinid)
	ContentHash string   `json:"content_hash"` // 内容哈希
	Metadata    string   `json:"metadata"`     // 元数据 (JSON 字符串)
	Disabled    bool     `json:"disabled"`     // 是否禁用

	// 链信息
	ChainName   string `json:"chain_name"`   // 链名称: btc, mvc
	BlockHeight int64  `json:"block_height"` // 区块高度
	Timestamp   int64  `json:"timestamp"`    // 时间戳

	// 创建者信息
	CreatorMetaId  string `json:"creator_meta_id"` // 创建者 MetaID
	CreatorAddress string `json:"creator_address"` // 创建者地址
	OwnerAddress   string `json:"owner_address"`   // 拥有者地址
	OwnerMetaId    string `json:"owner_meta_id"`   // 拥有者 MetaID

	// 状态信息
	Status int `json:"status"` // 状态: 0-失败, 1-成功
	State  int `json:"state"`  // 状态码

	// 时间戳
	CreatedAt time.Time `json:"created_at"` // 创建时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}
