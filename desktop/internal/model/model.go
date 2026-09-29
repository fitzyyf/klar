// 页面读的结果。字段和原来的 JSON 一致。
package model

type Output struct {
	Repo     string       `json:"repo"`
	Repos    []string     `json:"repos"`
	Projects []ProjectOut `json:"projects"`
	Sessions []SessionOut `json:"sessions"`
	Notes    []string     `json:"notes"`
}

type ProjectOut struct {
	Path   string `json:"path"`
	Claude uint32 `json:"claude"`
	Codex  uint32 `json:"codex"`
	Grok   uint32 `json:"grok"`
	Latest string `json:"latest"`
}

type SessionOut struct {
	Agent     string     `json:"agent"`
	Sid       string     `json:"sid"`
	Title     string     `json:"title"`
	Baseline  string     `json:"baseline"`
	Bound     string     `json:"bound"`
	Source    string     `json:"source"`
	Turns     []TurnOut  `json:"turns"`
	Net       TurnOut    `json:"net"`
	Review    *ReviewOut `json:"review,omitempty"`
	Assembled bool       `json:"assembled,omitempty"`
}

type ReviewOut struct {
	At      string `json:"at"`
	Verdict string `json:"verdict"`
	Text    string `json:"text"`
	Current bool   `json:"current"`
}

type TurnOut struct {
	ID      string     `json:"id"`
	At      string     `json:"at"`
	Prompt  string     `json:"prompt"`
	Reply   *string    `json:"reply,omitempty"`
	Edits   []EditOut  `json:"edits"`
	Files   []FileOut  `json:"files,omitempty"`
	Stars   []StarOut  `json:"stars"`
	Calls   []CallOut  `json:"calls"`
	Chains  []ChainOut `json:"chains"`
	Omitted int        `json:"omitted"`
	Errors  []ErrorOut `json:"errors"`
}

type FileOut struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type EditOut struct {
	Tool   string  `json:"tool"`
	Path   string  `json:"path"`
	Ok     bool    `json:"ok"`
	Reason *string `json:"reason,omitempty"`
}

type BodyOut struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type StarOut struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Kind         string   `json:"kind"`
	Mark         *string  `json:"mark,omitempty"`
	UnknownCalls *bool    `json:"unknownCalls,omitempty"`
	Body         *BodyOut `json:"body,omitempty"`
}

type CallOut struct {
	Key       string  `json:"key"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Change    string  `json:"change"`
	Uncertain *bool   `json:"uncertain,omitempty"`
	Before    *string `json:"before,omitempty"`
	After     *string `json:"after,omitempty"`
	Text      *string `json:"text,omitempty"`
	Guard     *string `json:"guard,omitempty"`
}

type ChainOut struct {
	Stars  []string `json:"stars"`
	Dashed *bool    `json:"dashed,omitempty"`
	Broken *string  `json:"broken,omitempty"`
}

type ErrorOut struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// VerdictOut 是记下结论的结果。Text 是要交回 agent 的那句话，界面给一键复制。
type VerdictOut struct {
	Ok   bool   `json:"ok"`
	Note string `json:"note"`
	Text string `json:"text"`
}
