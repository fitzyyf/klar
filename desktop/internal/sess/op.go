// 三家会话记录里的一次写文件。
package sess

type Op struct {
	Kind string // write add delete replace multi update
	Text string
	Old  string
	New  string
	All  bool
	Diff string
	List [][3]string // old, new, all("1"/"")
}

func Write(s string) Op     { return Op{Kind: "write", Text: s} }
func Add(s string) Op       { return Op{Kind: "add", Text: s} }
func Delete(s string) Op    { return Op{Kind: "delete", Text: s} }
func Update(diff string) Op { return Op{Kind: "update", Diff: diff} }
func Replace(old, new string, all bool) Op {
	return Op{Kind: "replace", Old: old, New: new, All: all}
}
func Multi(list [][3]string) Op { return Op{Kind: "multi", List: list} }

type RawEdit struct {
	Tool   string
	Path   string
	Op     Op
	Ok     bool
	Reason *string
	// Base: nil 表示记录里没有改前全文；指针指向 nil 表示当时没有这个文件。
	Base *Content
}

type Content = *string

type Snapshot map[string]Content

type RawTurn struct {
	At         string
	Prompt     string
	Reply      *string
	Edits      []RawEdit
	SnapBefore Snapshot
	SnapAfter  Snapshot
}

type RawSession struct {
	Source string
	Turns  []RawTurn
}
