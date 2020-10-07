package main

import (
	proto "chatDemo/proto"
	"context"
	"fmt"
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"google.golang.org/grpc"
	"io"
	"log"
	"regexp"
	"strings"
	"time"
)

type tClient struct {
	conn     *grpc.ClientConn
	stream   proto.Chat_BidStreamClient
	name     string
	app      *tview.Application
	login    *tview.Form
	chat     *tview.Flex
	content  *tview.TextView
	sayInput *tview.Form
}

const (
	MsgTypePrivate  = 1
	MsgTypeRoom     = 2
	MsgTypeSystem   = 3
	MsgTypeLoggedIn = 4
)

func (t tClient) quit() {
	t.app.Stop()
}

func (t tClient) getName() string {
	return t.name
}

func (t *tClient) LoginSubmit() {
	name := t.login.GetFormItem(0).(*tview.InputField).GetText()

	if strings.TrimSpace(name) == "" {
		t.modal("Please enter your name.", t.login)
		return
	}

	t.name = strings.ReplaceAll(name, " ", "_")
	if err := t.tryLogin(); err != nil {
		log.Println("login err.", err)
		return
	}

	t.initChatPanel()
	t.app.SetRoot(t.chat, true).SetFocus(t.chat)
}

func (t *tClient) initApp() *tview.Application {
	t.app = tview.NewApplication()
	t.initLogin()

	if err := t.app.SetRoot(t.login, true).SetFocus(t.login).Run(); err != nil {
		panic(err)
	}
	return t.app
}

func (t *tClient) modal(message string, primitive tview.Primitive) *tview.Modal {
	modal := tview.NewModal()
	modal.SetText(message).
		AddButtons([]string{"Ok"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Ok" {
				t.app.SetRoot(primitive, true).SetFocus(primitive)
			}
		})
	t.app.SetRoot(modal, false).SetFocus(modal)
	return modal
}

func (t *tClient) initLogin() {
	t.login = tview.NewForm()
	t.login.AddInputField("name", "", 20, nil, nil).
		AddButton("Submit", t.LoginSubmit).
		AddButton("Quit", t.quit)

	t.login.SetBorder(false).SetTitle(" Login ").SetTitleAlign(tview.AlignLeft)
}

func (t *tClient) initChatPanel() {
	t.chat = tview.NewFlex()
	t.content = tview.NewTextView().SetDynamicColors(true).SetRegions(true).SetChangedFunc(func() { t.app.Draw() })
	t.sayInput = tview.NewForm().AddInputField(t.getName()+":", "", 30, nil, nil)

	t.sayInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			say := t.sayInput.GetFormItem(0).(*tview.InputField).GetText()
			if strings.TrimSpace(say) != "" {
				t.sayInput.GetFormItem(0).(*tview.InputField).SetText("").GetFocusable()
				t.send(say)
			}
		}
		return event
	})
	//fmt.Fprintf(t.content, "%s", corporate)
	t.chat.AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(t.content, 0, 2, false).
		AddItem(t.sayInput, 5, 1, true), 0, 2, true)

	t.chat.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			if t.content.HasFocus() {
				t.app.SetFocus(t.chat)
			} else {
				t.app.SetFocus(t.content)
			}
		case tcell.KeyCtrlC:
			t.logout()
		}

		return event
	})
}

func (t *tClient) createStream() {
	conn, err := grpc.Dial("localhost:3020", grpc.WithInsecure())
	if err != nil {
		panic(fmt.Sprintf("connect err: [%v]", err))
		return
	}
	t.conn = conn
	client := proto.NewChatClient(t.conn)
	ctx := context.Background()
	stream, err := client.BidStream(ctx)
	if err != nil {
		panic(fmt.Sprintf("create stream err: [%v]", err))

		return
	}
	t.stream = stream
}

func (t tClient) tryLogin() error {
	if err := t.stream.Send(&proto.Request{Content: "may i login?", FromUser: t.getName(), Event: "login", RoomName: "defaultRoom"}); err != nil {
		fmt.Println("login err.", err)
		return err
	}
	return nil
}

func (t tClient) logout() {
	log.Println("logout.")

	if err := t.stream.Send(&proto.Request{Content: "bye bye.", FromUser: t.getName(), Event: "logout", RoomName: "defaultRoom"}); err != nil {
		log.Println(err)
		return
	}
	log.Println("logout..")

}

func (t tClient) send(string string) {
	r := regexp.MustCompile(`(?U)^@(.*)\s`)
	matches := r.FindStringSubmatch(string)
	request := proto.Request{Content: string, FromUser: t.getName(), Event: "msg", RoomName: "defaultRoom"}
	if len(matches) != 0 {
		request = proto.Request{Content: string, FromUser: t.getName(), Event: "msg", RoomName: "defaultRoom", ToUser: matches[1]}
	}

	if err := t.stream.Send(&request); err != nil {
		return
	}
}

func (t tClient) formatMsg(recv *proto.Response) string {
	var content string
	recvTime := time.Unix(recv.Time, 0).Format("15:04:05")
	switch recv.MsgType {
	case MsgTypePrivate:
		content = fmt.Sprintf("[gray]%s [red]%s[white]悄悄对你说: %v \n", recvTime, recv.FromUser, recv.Content)
	case MsgTypeSystem:
		content = fmt.Sprintf("[gray]%s [系统消息]: %v \n", recvTime, recv.Content)
	case MsgTypeRoom:
		content = fmt.Sprintf("[gray]%s [red]%s[white]: %v \n", recvTime, recv.FromUser, recv.Content)
	}

	return content
}

func main() {
	tClient := &tClient{}
	tClient.createStream()

	go func() {
		for {
			recv, err := tClient.stream.Recv()
			if err == io.EOF {
				log.Println("io EOF")
				return
			}

			if err != nil {
				log.Println("recv err:", err)
				return
			}

			if recv.MsgType == MsgTypeLoggedIn {
				tClient.modal(fmt.Sprintf("用户 %s 已登录，请更换用户名", tClient.getName()), tClient.login)
			}

			fmt.Fprintf(tClient.content, tClient.formatMsg(recv))
		}
	}()

	tClient.initApp()
	defer func() {
		log.Println("client closed.")
		if tClient.conn != nil {
			tClient.logout()
			defer tClient.conn.Close()
		}
	}()

}
