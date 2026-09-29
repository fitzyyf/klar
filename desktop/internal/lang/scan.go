package lang

import (
	"hash"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func (c *cx) isNamedFunc(n *sitter.Node) bool {
	switch c.kind {
	case Java:
		return oneOf(n.Kind(), "method_declaration", "constructor_declaration", "class_declaration")
	case Py:
		return oneOf(n.Kind(), "function_definition", "class_definition")
	case Go:
		return false
	default:
		if oneOf(n.Kind(), "function_declaration", "method_definition", "class_declaration") {
			return true
		}
		if !oneOf(n.Kind(), "arrow_function", "function_expression") {
			return false
		}
		p := n.Parent()
		return p != nil && oneOf(p.Kind(), "variable_declarator", "pair", "public_field_definition")
	}
}

func (c *cx) scan(n *sitter.Node, calls *[]Call, unknown *bool, h hash.Hash, root bool) {
	if !root && c.isNamedFunc(n) {
		return
	}
	if strings.Contains(n.Kind(), "comment") {
		return
	}
	if n.ChildCount() == 0 {
		_, _ = h.Write([]byte(c.text(n)))
		_, _ = h.Write([]byte{0x1f})
	}
	if call, ok := c.callOf(n, unknown); ok {
		*calls = append(*calls, call)
	}
	for _, child := range allKids(n) {
		c.scan(child, calls, unknown, h, false)
	}
}

func (c *cx) callOf(n *sitter.Node, unknown *bool) (Call, bool) {
	var name string
	var receiver *string
	switch c.kind {
	case Java:
		if n.Kind() != "method_invocation" {
			return Call{}, false
		}
		var ok bool
		name, ok = c.field(n, "name")
		if !ok {
			return Call{}, false
		}
		if obj, ok := c.field(n, "object"); ok {
			receiver = &obj
		}
	case Py:
		if n.Kind() != "call" {
			return Call{}, false
		}
		f := n.ChildByFieldName("function")
		if f == nil {
			return Call{}, false
		}
		switch f.Kind() {
		case "identifier":
			name = c.text(f)
		case "attribute":
			var ok bool
			name, ok = c.field(f, "attribute")
			if !ok {
				return Call{}, false
			}
			if obj, ok := c.field(f, "object"); ok {
				receiver = &obj
			}
		default:
			*unknown = true
			return Call{}, false
		}
	case Go:
		if n.Kind() != "call_expression" {
			return Call{}, false
		}
		f := n.ChildByFieldName("function")
		if f == nil {
			return Call{}, false
		}
		switch f.Kind() {
		case "identifier":
			name = c.text(f)
		case "selector_expression":
			var ok bool
			name, ok = c.field(f, "field")
			if !ok {
				return Call{}, false
			}
			if obj, ok := c.field(f, "operand"); ok {
				receiver = &obj
			}
		default:
			return Call{}, false
		}
	case Ts, Tsx, Js:
		if n.Kind() != "call_expression" {
			return Call{}, false
		}
		f := n.ChildByFieldName("function")
		if f == nil {
			return Call{}, false
		}
		switch f.Kind() {
		case "identifier":
			name = c.text(f)
		case "member_expression":
			var ok bool
			name, ok = c.field(f, "property")
			if !ok {
				return Call{}, false
			}
			if obj, ok := c.field(f, "object"); ok {
				receiver = &obj
			}
		case "subscript_expression":
			*unknown = true
			return Call{}, false
		default:
			return Call{}, false
		}
	default:
		return Call{}, false
	}
	switch name {
	case "apply", "call", "invoke", "getattr", "reflect":
		*unknown = true
		return Call{}, false
	}
	flat := strings.Join(strings.Fields(c.text(n)), " ")
	if len([]rune(flat)) > 160 {
		flat = string([]rune(flat)[:160])
	}
	return Call{Name: name, Receiver: receiver, Text: flat, Guard: c.guardOf(n)}, true
}

// guardOf 从这次调用往外看，经过的 if / else / switch / 循环 / 三元。条件本身里的调用不算被包着。
func (c *cx) guardOf(n *sitter.Node) *string {
	cur := n
	var parts []string
	for {
		parent := cur.Parent()
		if parent == nil {
			break
		}
		if c.isNamedFunc(parent) {
			break
		}
		if label, ok := c.judgment(parent, cur); ok {
			parts = append(parts, label)
		}
		cur = parent
	}
	if len(parts) == 0 {
		return nil
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	s := strings.Join(parts, "\u001e")
	return &s
}

func (c *cx) judgment(parent, child *sitter.Node) (string, bool) {
	kind := parent.Kind()
	if kind == "if_statement" {
		if c.inField(parent, "condition", child) {
			return "", false
		}
		cond := c.condText(parent)
		bare := bareCond(cond)
		decision := "if:" + cond
		if c.inField(parent, "alternative", child) {
			return piece(decision, "if", "else"), true
		}
		return piece(decision, "if", bare), true
	}
	if kind == "elif_clause" {
		cond := c.condText(parent)
		return piece("if:"+cond, "if", bareCond(cond)), true
	}
	if kind == "ternary_expression" || kind == "conditional_expression" {
		if c.inField(parent, "condition", child) {
			return "", false
		}
		cond := c.condText(parent)
		decision := "if:" + cond
		if c.inField(parent, "alternative", child) {
			return piece(decision, "if", "else"), true
		}
		return piece(decision, "if", bareCond(cond)), true
	}
	if oneOf(kind, "while_statement", "do_statement", "for_statement", "for_in_statement", "enhanced_for_statement") {
		if c.inField(parent, "condition", child) {
			return "", false
		}
		cond := c.condText(parent)
		kw := "while"
		if strings.HasPrefix(kind, "for") {
			kw = "for"
		}
		return piece(kw+":"+cond, kw, bareCond(cond)), true
	}
	if oneOf(kind, "switch_case", "switch_default", "expression_case", "default_case", "type_case", "switch_label") {
		key := c.switchKey(parent)
		branch := c.headLine(parent)
		branch = strings.TrimSpace(strings.TrimSuffix(branch, ":"))
		if strings.Contains(kind, "default") {
			branch = "default"
		}
		return piece("switch:"+key, "switch", branch), true
	}
	if kind == "catch_clause" || kind == "except_clause" {
		label := c.headLine(parent)
		return piece("catch:"+label, "catch", label), true
	}
	return "", false
}

func piece(decision, kind, branch string) string {
	clean := func(s string) string {
		s = strings.ReplaceAll(s, "\u0001", " ")
		return strings.ReplaceAll(s, "\u001e", " ")
	}
	return clean(decision) + "\u0001" + kind + "\u0001" + clean(branch)
}

func bareCond(cond string) string {
	t := strings.TrimSpace(cond)
	if len(t) > 2 && strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") {
		return strings.TrimSpace(t[1 : len(t)-1])
	}
	return t
}

func (c *cx) switchKey(caseNode *sitter.Node) string {
	cur := caseNode
	for {
		parent := cur.Parent()
		if parent == nil {
			return c.headLine(caseNode)
		}
		k := parent.Kind()
		if strings.Contains(k, "switch") && !strings.Contains(k, "case") && !strings.Contains(k, "label") && !strings.Contains(k, "default") {
			return c.condText(parent)
		}
		cur = parent
	}
}

func (c *cx) inField(n *sitter.Node, field string, child *sitter.Node) bool {
	got := n.ChildByFieldName(field)
	if got == nil {
		return false
	}
	return got.Id() == child.Id() || contains(got, child)
}

func contains(outer, inner *sitter.Node) bool {
	cur := inner
	for {
		parent := cur.Parent()
		if parent == nil {
			return false
		}
		if parent.Id() == outer.Id() {
			return true
		}
		cur = parent
	}
}

func (c *cx) condText(n *sitter.Node) string {
	if cond := n.ChildByFieldName("condition"); cond != nil {
		return squash(c.text(cond))
	}
	for _, child := range allKids(n) {
		if !child.IsNamed() {
			continue
		}
		if oneOf(child.Kind(), "statement_block", "block", "else_clause", "elif_clause", "if_statement") {
			continue
		}
		return squash(c.text(child))
	}
	return ""
}

func (c *cx) headLine(n *sitter.Node) string {
	text := c.text(n)
	head, _, _ := strings.Cut(text, "\n")
	return squash(head)
}

func squash(raw string) string {
	flat := strings.Join(strings.Fields(raw), " ")
	r := []rune(flat)
	if len(r) > 48 {
		r = r[:48]
	}
	return string(r)
}

// Decode 拆开 guard 字符串。给测试和编链用。
type Step struct {
	Decision string
	Kind     string
	Branch   string
}

func Decode(raw string) []Step {
	if raw == "" {
		return nil
	}
	var out []Step
	for _, part := range strings.Split(raw, "\u001e") {
		bits := strings.Split(part, "\u0001")
		if len(bits) < 3 {
			continue
		}
		out = append(out, Step{Decision: bits[0], Kind: bits[1], Branch: bits[2]})
	}
	return out
}
