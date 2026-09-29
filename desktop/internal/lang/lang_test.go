package lang

import (
	"strings"
	"testing"
)

func TestJSGuards(t *testing.T) {
	src := `
            function create(order) {
              if (order.price < 0) {
                charge(order);
              } else {
                reserve(order);
              }
              audit(order);
              while (order.left > 0) {
                tick(order);
              }
              switch (order.status) {
                case "paid":
                  ship(order);
                  break;
                default:
                  cancel(order);
              }
            }
        `
	funcs := Parse("a.js", src)
	var create *Func
	for i := range funcs {
		if funcs[i].Name == "create" {
			create = &funcs[i]
		}
	}
	if create == nil {
		t.Fatal("没有 create")
	}
	steps := func(name string) []Step {
		for _, c := range create.Calls {
			if c.Name == name && c.Guard != nil {
				return Decode(*c.Guard)
			}
		}
		return nil
	}
	charge, reserve := steps("charge"), steps("reserve")
	if len(charge) != 1 || charge[0].Kind != "if" || !strings.Contains(charge[0].Branch, "price") {
		t.Fatalf("charge %#v", charge)
	}
	if len(reserve) != 1 || reserve[0].Branch != "else" {
		t.Fatalf("reserve %#v", reserve)
	}
	if charge[0].Decision != reserve[0].Decision {
		t.Fatalf("if/else 不是同一个判断 %q %q", charge[0].Decision, reserve[0].Decision)
	}
	if len(steps("audit")) != 0 {
		t.Fatal("audit 不该被判断包着")
	}
	if len(steps("tick")) != 1 || steps("tick")[0].Kind != "while" {
		t.Fatalf("tick %#v", steps("tick"))
	}
	ship, cancel := steps("ship"), steps("cancel")
	if len(ship) != 1 || !strings.Contains(ship[0].Branch, "paid") {
		t.Fatalf("ship %#v", ship)
	}
	if len(cancel) != 1 || cancel[0].Branch != "default" {
		t.Fatalf("cancel %#v", cancel)
	}
	if ship[0].Decision != cancel[0].Decision {
		t.Fatalf("switch 分支不是同一个判断")
	}
}

