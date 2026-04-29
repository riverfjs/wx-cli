package api

// iLink protocol types — mirrors vendor/openclaw-weixin/src/api/types.ts

type Credential struct {
	BotToken  string `json:"bot_token"`
	BaseURL   string `json:"baseurl"`
	BotID     string `json:"ilink_bot_id"`
	UserID    string `json:"ilink_user_id"`
	LoginTime string `json:"login_time"`
}

type BaseInfo struct {
	ChannelVersion string `json:"channel_version"`
}

type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
}

type TextItem struct {
	Text string `json:"text,omitempty"`
}

type ImageItem struct {
	Media   *CDNMedia `json:"media,omitempty"`
	MidSize int       `json:"mid_size,omitempty"`
}

type FileItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	Len      string    `json:"len,omitempty"`
}

type VideoItem struct {
	Media     *CDNMedia `json:"media,omitempty"`
	VideoSize int       `json:"video_size,omitempty"`
}

type VoiceItem struct {
	Media    *CDNMedia `json:"media,omitempty"`
	Playtime int       `json:"playtime,omitempty"`
	Text     string    `json:"text,omitempty"`
}

type MessageItem struct {
	Type      int        `json:"type,omitempty"`
	TextItem  *TextItem  `json:"text_item,omitempty"`
	ImageItem *ImageItem `json:"image_item,omitempty"`
	FileItem  *FileItem  `json:"file_item,omitempty"`
	VideoItem *VideoItem `json:"video_item,omitempty"`
	VoiceItem *VoiceItem `json:"voice_item,omitempty"`
}

type WeixinMessage struct {
	Seq          int            `json:"seq,omitempty"`
	MessageID    int64          `json:"message_id,omitempty"`
	FromUserID   string         `json:"from_user_id"`
	ToUserID     string         `json:"to_user_id,omitempty"`
	ClientID     string         `json:"client_id,omitempty"`
	CreateTimeMs int64          `json:"create_time_ms,omitempty"`
	MessageType  int            `json:"message_type,omitempty"`
	MessageState int            `json:"message_state,omitempty"`
	ItemList     []*MessageItem `json:"item_list,omitempty"`
	ContextToken string         `json:"context_token,omitempty"`
}

type GetUpdatesResp struct {
	Ret           *int             `json:"ret,omitempty"`
	ErrCode       *int             `json:"errcode,omitempty"`
	ErrMsg        string           `json:"errmsg,omitempty"`
	Msgs          []*WeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf string           `json:"get_updates_buf,omitempty"`
}

type UploadURLResp struct {
	Ret           *int   `json:"ret,omitempty"`
	UploadParam   string `json:"upload_param,omitempty"`
	UploadFullURL string `json:"upload_full_url,omitempty"`
}
