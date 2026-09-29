// 切出函数、调用点和函数体哈希。不接语言服务器。
package lang

import (
	"hash/fnv"
	"strings"
	"sync"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tsjava "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tspy "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

type Kind int

const (
	Java Kind = iota
	Ts
	Tsx
	Js
	Py
	Go
)

type Call struct {
	Name     string
	Receiver *string
	Text     string
	// 包着这次调用的判断，从外到内。
	Guard *string
}

type Func struct {
	ID            string
	Path          string
	Container     string
	Name          string
	Hash          uint64
	Body          string
	Calls         []Call
	Entry         bool
	Abstract      bool
	Supers        []string
	UnknownCalls  bool
}

func LangOf(path string) (Kind, bool) {
	ext := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		ext = strings.ToLower(path[i+1:])
	}
	switch ext {
	case "java":
		return Java, true
	case "ts", "mts", "cts":
		return Ts, true
	case "tsx":
		return Tsx, true
	case "js", "jsx", "mjs", "cjs":
		return Js, true
	case "vue":
		return Ts, true
	case "py":
		return Py, true
	case "go":
		return Go, true
	default:
		return 0, false
	}
}

func grammar(k Kind) *sitter.Language {
	switch k {
	case Java:
		return sitter.NewLanguage(tsjava.Language())
	case Ts:
		return sitter.NewLanguage(tsts.LanguageTypescript())
	case Tsx:
		return sitter.NewLanguage(tsts.LanguageTSX())
	case Js:
		return sitter.NewLanguage(tsjs.Language())
	case Py:
		return sitter.NewLanguage(tspy.Language())
	default:
		return sitter.NewLanguage(tsgo.Language())
	}
}

var parsers sync.Mutex

// ModuleName 是模块级函数挂靠的名字。index.* 用所在目录名。
func ModuleName(path string) string {
	parts := strings.Split(path, "/")
	file := parts[len(parts)-1]
	stem := file
	if i := strings.Index(file, "."); i >= 0 {
		stem = file[:i]
	}
	switch stem {
	case "index", "main", "mod", "__init__":
		if len(parts) >= 2 {
			return parts[len(parts)-2]
		}
	}
	return stem
}

func vueScript(src string) string {
	open := strings.Index(src, "<script")
	if open < 0 {
		return ""
	}
	gt := strings.Index(src[open:], ">")
	if gt < 0 {
		return ""
	}
	start := open + gt + 1
	end := len(src)
	if rel := strings.Index(src[start:], "</script>"); rel >= 0 {
		end = start + rel
	}
	return strings.Repeat("\n", strings.Count(src[:start], "\n")) + src[start:end]
}

// Parse 切出一个文件里的函数。语法对不上就返回空。
func Parse(path, src string) []Func {
	kind, ok := LangOf(path)
	if !ok {
		return nil
	}
	code := src
	if strings.HasSuffix(path, ".vue") {
		code = vueScript(src)
	}
	parsers.Lock()
	parser := sitter.NewParser()
	if err := parser.SetLanguage(grammar(kind)); err != nil {
		parser.Close()
		parsers.Unlock()
		return nil
	}
	tree := parser.Parse([]byte(code), nil)
	parsers.Unlock()
	if tree == nil {
		parser.Close()
		return nil
	}
	cx := &cx{kind: kind, src: []byte(code), path: path}
	cx.walk(tree.RootNode(), ModuleName(path), nil)
	tree.Close()
	parser.Close()
	return dedupe(cx.out)
}

func dedupe(in []Func) []Func {
	seen := map[string]int{}
	for i := range in {
		id := in[i].ID
		n := seen[id]
		if n > 0 {
			in[i].ID = id + "~" + itoa(n)
		}
		seen[id] = n + 1
	}
	return in
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

type cx struct {
	kind Kind
	src  []byte
	path string
	out  []Func
}

func (c *cx) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Utf8Text(c.src)
}

func (c *cx) field(n *sitter.Node, name string) (string, bool) {
	ch := n.ChildByFieldName(name)
	if ch == nil {
		return "", false
	}
	return c.text(ch), true
}

func named(n *sitter.Node) []*sitter.Node {
	if n == nil {
		return nil
	}
	cur := n.Walk()
	defer cur.Close()
	kids := n.NamedChildren(cur)
	out := make([]*sitter.Node, len(kids))
	for i := range kids {
		out[i] = &kids[i]
	}
	return out
}

func allKids(n *sitter.Node) []*sitter.Node {
	if n == nil {
		return nil
	}
	cur := n.Walk()
	defer cur.Close()
	kids := n.Children(cur)
	out := make([]*sitter.Node, len(kids))
	for i := range kids {
		out[i] = &kids[i]
	}
	return out
}

func (c *cx) walk(node *sitter.Node, container string, supers []string) {
	for _, child := range named(node) {
		if name, next, ok := c.classLike(child); ok {
			c.walk(child, name, next)
			continue
		}
		if f, ok := c.function(child, container, supers); ok {
			c.out = append(c.out, f)
		}
		c.walk(child, container, supers)
	}
}

func (c *cx) classLike(n *sitter.Node) (string, []string, bool) {
	kind := n.Kind()
	isClass := false
	switch c.kind {
	case Java:
		isClass = oneOf(kind, "class_declaration", "interface_declaration", "enum_declaration", "record_declaration")
	case Ts, Tsx, Js:
		isClass = oneOf(kind, "class_declaration", "abstract_class_declaration", "class", "interface_declaration")
	case Py:
		isClass = kind == "class_definition"
	}
	if !isClass {
		return "", nil, false
	}
	name, ok := c.field(n, "name")
	if !ok {
		return "", nil, false
	}
	var supers []string
	stack := []*sitter.Node{n}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, ch := range named(x) {
			switch ch.Kind() {
			case "class_body", "interface_body", "enum_body", "block":
			case "type_identifier", "identifier":
				if x.Id() != n.Id() {
					supers = append(supers, c.text(ch))
				}
			case "superclass", "super_interfaces", "extends_interfaces", "type_list", "class_heritage", "extends_clause", "implements_clause", "argument_list", "generic_type", "extends_type_clause":
				stack = append(stack, ch)
			}
		}
	}
	return name, supers, true
}

func oneOf(s string, opts ...string) bool {
	for _, o := range opts {
		if s == o {
			return true
		}
	}
	return false
}

func (c *cx) function(n *sitter.Node, container string, supers []string) (Func, bool) {
	kind := n.Kind()
	var name string
	var params int
	var body *sitter.Node
	var entry bool
	var owner string
	switch c.kind {
	case Java:
		if kind != "method_declaration" && kind != "constructor_declaration" {
			return Func{}, false
		}
		var ok bool
		name, ok = c.field(n, "name")
		if !ok {
			return Func{}, false
		}
		if p := n.ChildByFieldName("parameters"); p != nil {
			for _, x := range named(p) {
				if x.Kind() == "formal_parameter" || x.Kind() == "spread_parameter" {
					params++
				}
			}
		}
		for _, m := range named(n) {
			if m.Kind() != "modifiers" {
				continue
			}
			t := c.text(m)
			for _, k := range []string{"Mapping(", "Mapping\n", "Mapping ", "@Scheduled", "@KafkaListener", "@RabbitListener", "@EventListener", "@XxlJob", "@JmsListener"} {
				if strings.Contains(t, k) {
					entry = true
				}
			}
			if strings.HasSuffix(t, "Mapping") {
				entry = true
			}
			if name == "main" && strings.Contains(t, "static") {
				entry = true
			}
		}
		body = n.ChildByFieldName("body")
		owner = container
	case Ts, Tsx, Js:
		f := n
		switch kind {
		case "function_declaration", "generator_function_declaration", "method_definition", "method_signature", "abstract_method_signature":
			var ok bool
			name, ok = c.field(n, "name")
			if !ok {
				return Func{}, false
			}
		case "variable_declarator", "public_field_definition", "field_definition", "pair", "assignment_expression":
			value := n.ChildByFieldName("value")
			if value == nil {
				value = n.ChildByFieldName("right")
			}
			if value == nil || !oneOf(value.Kind(), "arrow_function", "function_expression", "function") {
				return Func{}, false
			}
			key := n.ChildByFieldName("name")
			if key == nil {
				key = n.ChildByFieldName("key")
			}
			if key == nil {
				key = n.ChildByFieldName("property")
			}
			if key == nil {
				key = n.ChildByFieldName("left")
			}
			if key == nil {
				return Func{}, false
			}
			raw := c.text(key)
			if i := strings.LastIndex(raw, "."); i >= 0 {
				raw = raw[i+1:]
			}
			name = strings.Trim(raw, `"'`)
			f = value
		default:
			return Func{}, false
		}
		if p := f.ChildByFieldName("parameters"); p != nil {
			params = int(p.NamedChildCount())
		} else if f.ChildByFieldName("parameter") != nil {
			params = 1
		}
		entry = oneOf(name, "mounted", "created", "setup", "onMounted")
		body = f.ChildByFieldName("body")
		owner = container
	case Py:
		if kind != "function_definition" {
			return Func{}, false
		}
		var ok bool
		name, ok = c.field(n, "name")
		if !ok {
			return Func{}, false
		}
		if p := n.ChildByFieldName("parameters"); p != nil {
			for _, x := range named(p) {
				t := c.text(x)
				if t != "self" && t != "cls" {
					params++
				}
			}
		}
		if parent := n.Parent(); parent != nil && parent.Kind() == "decorated_definition" {
			for _, d := range named(parent) {
				if d.Kind() != "decorator" {
					continue
				}
				t := c.text(d)
				for _, k := range []string{".get", ".post", ".put", ".delete", ".patch", ".route", ".websocket", "task", "listener", "command"} {
					if strings.Contains(t, k) {
						entry = true
					}
				}
			}
		}
		body = n.ChildByFieldName("body")
		owner = container
	case Go:
		if kind != "function_declaration" && kind != "method_declaration" {
			return Func{}, false
		}
		var ok bool
		name, ok = c.field(n, "name")
		if !ok {
			return Func{}, false
		}
		if p := n.ChildByFieldName("parameters"); p != nil {
			for _, d := range named(p) {
				idents := 0
				for _, x := range named(d) {
					if x.Kind() == "identifier" {
						idents++
					}
				}
				if idents < 1 {
					idents = 1
				}
				params += idents
			}
		}
		owner = container
		if r := n.ChildByFieldName("receiver"); r != nil {
			t := strings.Trim(c.text(r), "()")
			fields := strings.Fields(t)
			if len(fields) > 0 {
				last := fields[len(fields)-1]
				last = strings.TrimPrefix(last, "*")
				if i := strings.Index(last, "["); i >= 0 {
					last = last[:i]
				}
				if last != "" {
					owner = last
				}
			}
		}
		entry = name == "main" || name == "ServeHTTP"
		body = n.ChildByFieldName("body")
	}
	var calls []Call
	unknown := false
	h := fnv.New64a()
	bodyText := ""
	if body != nil {
		bodyText = c.text(body)
		c.scan(body, &calls, &unknown, h, true)
	}
	return Func{
		ID: c.path + "#" + owner + "#" + name + "#" + itoa(params), Path: c.path, Container: owner, Name: name,
		Hash: h.Sum64(), Body: bodyText, Calls: calls, Entry: entry, Abstract: body == nil,
		Supers: append([]string(nil), supers...), UnknownCalls: unknown,
	}, true
}
