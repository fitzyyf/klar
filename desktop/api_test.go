package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"klar.dev/desktop/internal/engine"
	"klar.dev/desktop/internal/model"
)

func TestPageAndBadVerdict(t *testing.T) {
	ui, err := uiDir()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler(ui, engine.New()))
	defer srv.Close()

	page, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer page.Body.Close()
	html, _ := io.ReadAll(page.Body)
	if page.StatusCode != 200 || !strings.Contains(string(html), "Agent会话Review") {
		t.Fatalf("页面 status=%d", page.StatusCode)
	}

	res, err := http.Post(srv.URL+"/api/verdict", "application/json", strings.NewReader("{"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), `"ok":false`) {
		t.Fatalf("坏评审 status=%d body=%s", res.StatusCode, body)
	}
}

// 界面拿到的 text 就是粘给 agent 的那句话，通话的这句里不能出现投递相关的字眼。
func TestVerdictHandsBackCopyableText(t *testing.T) {
	ui, err := uiDir()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	app := engine.WithHome(dir)
	srv := httptest.NewServer(handler(ui, app))
	defer srv.Close()

	send := func(payload any) model.VerdictOut {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.Post(srv.URL+"/api/verdict", "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var out model.VerdictOut
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	back := send(map[string]string{"repo": dir, "sid": "s1", "verdict": "back", "opinion": "判断写反了", "title": "改价格", "turnId": "t2"})
	if !back.Ok || !strings.HasPrefix(back.Text, "评审没通过：判断写反了。") {
		t.Fatalf("%+v", back)
	}
	if strings.Contains(back.Text, "已发给") || strings.Contains(back.Text, "没发出去") {
		t.Fatalf("交回的话里混进了投递结果：%s", back.Text)
	}
	pass := send(map[string]string{"repo": dir, "sid": "s1", "verdict": "pass", "title": "改价格", "turnId": "t2"})
	if !pass.Ok || !strings.Contains(pass.Text, "提交说明：改价格。") || !strings.Contains(pass.Text, "只提交这个会话写过的文件。") {
		t.Fatalf("%+v", pass)
	}
	if pass.Note == "" || !strings.Contains(pass.Note, "复制") {
		t.Fatalf("界面没被告知要自己粘：%s", pass.Note)
	}
}
