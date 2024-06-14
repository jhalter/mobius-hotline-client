package ui

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/jhalter/mobius/hotline"
	"github.com/rivo/tview"
	"math/big"
	"strings"
	"time"
)

//var DefaultClientHandlers = map[uint16]hotline.ClientHandler{
//	hotline.TranChatMsg:          handleClientChatMsg,
//	hotline.TranLogin:            handleClientTranLogin,
//	hotline.TranShowAgreement:    handleClientTranShowAgreement,
//	hotline.TranUserAccess:       handleClientTranUserAccess,
//	hotline.TranGetUserNameList:  handleClientGetUserNameList,
//	hotline.TranNotifyChangeUser: handleNotifyChangeUser,
//	hotline.TranNotifyDeleteUser: handleNotifyDeleteUser,
//	hotline.TranGetMsgs:          handleGetMsgs,
//	hotline.TranGetFileNameList:  handleGetFileNameList,
//	hotline.TranServerMsg:        handleTranServerMsg,
//	hotline.TranKeepAlive: func(ctx context.Context, client *hotline.Client, transaction *hotline.Transaction) (t []hotline.Transaction, err error) {
//		return t, err
//	},
//}

func (mhc *Client) HandleKeepAlive(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	return res, err
}

func (mhc *Client) HandleTranServerMsg(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	now := time.Now().Format(time.RFC850)

	msg := strings.ReplaceAll(string(t.GetField(hotline.FieldData).Data), "\r", "\n")
	msg += "\n\nAt " + now
	title := fmt.Sprintf("| Private Message From: 	%s |", t.GetField(hotline.FieldUserName).Data)

	msgBox := tview.NewTextView().SetScrollable(true)
	msgBox.SetText(msg).SetBackgroundColor(tcell.ColorDarkSlateBlue)
	msgBox.SetTitle(title).SetBorder(true)
	msgBox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			mhc.UI.Pages.RemovePage("serverMsgModal" + now)
		}
		return event
	})

	centeredFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(msgBox, 0, 2, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	mhc.UI.Pages.AddPage("serverMsgModal"+now, centeredFlex, true, true)
	mhc.UI.App.Draw() // TODO: errModal doesn't render without this.  wtf?

	return res, err
}

func (mhc *Client) showErrMsg(msg string) {
	t := time.Now().Format(time.RFC850)

	title := "| Error |"

	msgBox := tview.NewTextView().SetScrollable(true)
	msgBox.SetText(msg).SetBackgroundColor(tcell.ColorDarkRed)
	msgBox.SetTitle(title).SetBorder(true)
	msgBox.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			mhc.UI.Pages.RemovePage("serverMsgModal" + t)
		}
		return event
	})

	centeredFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(msgBox, 0, 2, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	mhc.UI.Pages.AddPage("serverMsgModal"+t, centeredFlex, true, true)
	mhc.UI.App.Draw() // TODO: errModal doesn't render without this.  wtf?
}

func (mhc *Client) HandleGetFileNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if t.IsError() {
		mhc.showErrMsg(string(t.GetField(hotline.FieldError).Data))
		return res, err
	}

	fTree := tview.NewTreeView().SetTopLevel(1)
	root := tview.NewTreeNode("Root")
	fTree.SetRoot(root).SetCurrentNode(root)
	fTree.SetBorder(true).SetTitle("| Files |")
	fTree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			mhc.UI.Pages.RemovePage("files")
			mhc.filePath = []string{}
		case tcell.KeyEnter:
			selectedNode := fTree.GetCurrentNode()

			if selectedNode.GetText() == "<- Back" {
				mhc.filePath = mhc.filePath[:len(mhc.filePath)-1]
				f := hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(strings.Join(mhc.filePath, "/")))

				if err := mhc.UI.HLClient.Send(*hotline.NewTransaction(hotline.TranGetFileNameList, nil, f)); err != nil {
					mhc.UI.HLClient.Logger.Error("err", "err", err)
				}
				return event
			}

			entry := selectedNode.GetReference().(*hotline.FileNameWithInfo)

			if bytes.Equal(entry.Type[:], []byte("fldr")) {
				c.Logger.Info("get new directory listing", "name", string(entry.Name))

				mhc.filePath = append(mhc.filePath, string(entry.Name))
				f := hotline.NewField(hotline.FieldFilePath, hotline.EncodeFilePath(strings.Join(mhc.filePath, "/")))

				if err := mhc.UI.HLClient.Send(*hotline.NewTransaction(hotline.TranGetFileNameList, nil, f)); err != nil {
					mhc.UI.HLClient.Logger.Error("err", "err", err)
				}
			} else {
				// TODO: initiate file download
				c.Logger.Info("download file", "name", string(entry.Name))
			}
		}

		return event
	})

	if len(mhc.filePath) > 0 {
		node := tview.NewTreeNode("<- Back")
		root.AddChild(node)
	}

	for _, f := range t.Fields {
		var fn hotline.FileNameWithInfo
		_, err = fn.Write(f.Data)
		if err != nil {
			return nil, nil
		}

		if bytes.Equal(fn.Type[:], []byte("fldr")) {
			node := tview.NewTreeNode(fmt.Sprintf("[blue::]📁 %s[-:-:-]", fn.Name))
			node.SetReference(&fn)
			root.AddChild(node)
		} else {
			size := binary.BigEndian.Uint32(fn.FileSize[:]) / 1024

			node := tview.NewTreeNode(fmt.Sprintf("   %-40s %10v KB", fn.Name, size))
			node.SetReference(&fn)
			root.AddChild(node)
		}
	}

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(fTree, 20, 1, true).
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	mhc.UI.Pages.AddPage("files", centerFlex, true, true)
	mhc.UI.App.Draw()

	return res, err
}

func (mhc *Client) TranGetMsgs(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	newsText := string(t.GetField(hotline.FieldData).Data)
	newsText = strings.ReplaceAll(newsText, "\r", "\n")

	newsTextView := tview.NewTextView().
		SetText(newsText).
		SetDoneFunc(func(key tcell.Key) {
			mhc.UI.Pages.SwitchToPage(serverUIPage)
			mhc.UI.App.SetFocus(mhc.UI.chatInput)
		})
	newsTextView.SetBorder(true).SetTitle("News")

	mhc.UI.Pages.AddPage("news", newsTextView, true, true)
	// mhc.UI.Pages.SwitchToPage("news")
	// mhc.UI.App.SetFocus(newsTextView)
	mhc.UI.App.Draw()

	return res, err
}

func (mhc *Client) HandleNotifyChangeUser(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	newUser := hotline.User{
		ID:    t.GetField(hotline.FieldUserID).Data,
		Name:  string(t.GetField(hotline.FieldUserName).Data),
		Icon:  t.GetField(hotline.FieldUserIconID).Data,
		Flags: t.GetField(hotline.FieldUserFlags).Data,
	}

	// Possible cases:
	// user is new to the server
	// user is already on the server but has a new name

	var oldName string
	var newUserList []hotline.User
	updatedUser := false
	for _, u := range c.UserList {
		if bytes.Equal(newUser.ID, u.ID) {
			oldName = u.Name
			u.Name = newUser.Name
			if u.Name != newUser.Name {
				_, _ = fmt.Fprintf(mhc.UI.chatBox, " <<< "+oldName+" is now known as "+newUser.Name+" >>>\n")
			}
			updatedUser = true
		}
		newUserList = append(newUserList, u)
	}

	if !updatedUser {
		newUserList = append(newUserList, newUser)
	}

	c.UserList = newUserList

	mhc.renderUserList()

	return res, err
}

func (mhc *Client) HandleNotifyDeleteUser(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	exitUser := t.GetField(hotline.FieldUserID).Data

	var newUserList []hotline.User
	for _, u := range c.UserList {
		if !bytes.Equal(exitUser, u.ID) {
			newUserList = append(newUserList, u)
		}
	}

	c.UserList = newUserList

	mhc.renderUserList()

	return res, err
}

func (mhc *Client) HandleClientGetUserNameList(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	var users []hotline.User
	for _, field := range t.Fields {
		// The Hotline protocol docs say that ClientGetUserNameList should only return hotline.FieldUserNameWithInfo (300)
		// fields, but shxd sneaks in FieldChatSubject (115) so it's important to filter explicitly for the expected
		// field type.  Probably a good idea to do everywhere.
		if field.ID == [2]byte{0x01, 0x2c} {
			var user hotline.User
			if _, err := user.Write(field.Data); err != nil {
				return res, fmt.Errorf("unable to read user data: %w", err)
			}

			users = append(users, user)
		}
	}
	mhc.UserList = users

	mhc.renderUserList()

	return res, err
}

func (mhc *Client) renderUserList() {
	mhc.UI.userList.Clear()
	for _, u := range mhc.UserList {
		flagBitmap := big.NewInt(int64(binary.BigEndian.Uint16(u.Flags)))
		if flagBitmap.Bit(hotline.UserFlagAdmin) == 1 {
			_, _ = fmt.Fprintf(mhc.UI.userList, "[red::b]%s[-:-:-]\n", u.Name)
		} else {
			_, _ = fmt.Fprintf(mhc.UI.userList, "%s\n", u.Name)
		}
		// TODO: fade if user is away
	}
}

func (mhc *Client) HandleClientChatMsg(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if c.Pref.EnableBell {
		fmt.Println("\a")
	}

	_, _ = fmt.Fprintf(mhc.UI.chatBox, "%s \n", t.GetField(hotline.FieldData).Data)

	return res, err
}

func (mhc *Client) HandleClientTranUserAccess(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	c.UserAccess = t.GetField(hotline.FieldUserAccess).Data

	return res, err
}

func (mhc *Client) HandleClientTranShowAgreement(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	agreement := string(t.GetField(hotline.FieldData).Data)
	agreement = strings.ReplaceAll(agreement, "\r", "\n")

	agreeModal := tview.NewModal().
		SetText(agreement).
		AddButtons([]string{"Agree", "Disagree"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonIndex == 0 {
				res = append(res,
					*hotline.NewTransaction(
						hotline.TranAgreed, nil,
						hotline.NewField(hotline.FieldUserName, []byte(c.Pref.Username)),
						hotline.NewField(hotline.FieldUserIconID, c.Pref.IconBytes()),
						hotline.NewField(hotline.FieldUserFlags, []byte{0x00, 0x00}),
						hotline.NewField(hotline.FieldOptions, []byte{0x00, 0x00}),
					),
				)
				mhc.UI.Pages.HidePage("agreement")
				mhc.UI.App.SetFocus(mhc.UI.chatInput)
			} else {
				_ = c.Disconnect()
				mhc.UI.Pages.SwitchToPage("home")
			}
		},
		)

	mhc.UI.Pages.AddPage("agreement", agreeModal, false, true)

	return res, err
}

func (mhc *Client) HandleClientTranLogin(ctx context.Context, c *hotline.Client, t *hotline.Transaction) (res []hotline.Transaction, err error) {
	if !bytes.Equal(t.ErrorCode, []byte{0, 0, 0, 0}) {
		errMsg := string(t.GetField(hotline.FieldError).Data)
		errModal := tview.NewModal()
		errModal.SetText(errMsg)
		errModal.AddButtons([]string{"Oh no"})
		errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			mhc.UI.Pages.RemovePage("errModal")
		})
		mhc.UI.Pages.RemovePage("joinServer")
		mhc.UI.Pages.AddPage("errModal", errModal, false, true)

		mhc.UI.App.Draw() // TODO: errModal doesn't render without this.  wtf?

		c.Logger.Error(string(t.GetField(hotline.FieldError).Data))
		return nil, errors.New("login error: " + string(t.GetField(hotline.FieldError).Data))
	}
	mhc.UI.Pages.AddAndSwitchToPage(serverUIPage, mhc.UI.renderServerUI(), true)
	mhc.UI.App.SetFocus(mhc.UI.chatInput)

	if err := c.Send(*hotline.NewTransaction(hotline.TranGetUserNameList, nil)); err != nil {
		c.Logger.Error("err", "err", err)
	}
	return res, err
}
