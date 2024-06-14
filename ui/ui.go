package ui

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/jhalter/mobius/hotline"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

// DebugBuffer wraps a *tview.TextView and adds a Sync() method to make it available as a Zap logger
type DebugBuffer struct {
	TextView *tview.TextView
}

func (db *DebugBuffer) Write(p []byte) (int, error) {
	return db.TextView.Write(p)
}

//// Sync is a noop function that dataFile to satisfy the zapcore.WriteSyncer interface
//func (db *DebugBuffer) Sync() error {
//	return nil
//}

type Bookmark struct {
	Name     string `yaml:"Name"`
	Addr     string `yaml:"Addr"`
	Login    string `yaml:"Login"`
	Password string `yaml:"Password"`
}

type ClientPrefs struct {
	Username   string     `yaml:"Username"`
	IconID     int        `yaml:"IconID"`
	Bookmarks  []Bookmark `yaml:"Bookmarks"`
	Tracker    string     `yaml:"Tracker"`
	EnableBell bool       `yaml:"EnableBell"`
}

func (cp *ClientPrefs) IconBytes() []byte {
	iconBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(iconBytes, uint16(cp.IconID))
	return iconBytes
}

func (cp *ClientPrefs) AddBookmark(name, addr, login, pass string) {
	cp.Bookmarks = append(cp.Bookmarks, Bookmark{Addr: addr, Login: login, Password: pass})
}

type Client struct {
	CfgPath     string
	DebugBuf    *DebugBuffer
	Connection  net.Conn
	UserAccess  []byte
	filePath    []string
	UserList    []hotline.User
	Logger      *slog.Logger
	activeTasks map[uint32]*hotline.Transaction
	ServerName  string

	Pref *ClientPrefs

	Handlers map[uint16]hotline.ClientHandler

	UI *UI

	Inbox chan *hotline.Transaction
}

type UI struct {
	chatBox     *tview.TextView
	chatInput   *tview.InputField
	App         *tview.Application
	Pages       *tview.Pages
	userList    *tview.TextView
	trackerList *tview.List
	Pref        *ClientPrefs
	HLClient    *hotline.Client
}

// pages
const (
	pageServerUI    = "serverUI"
	trackerListPage = "trackerList"
	serverUIPage    = "serverUI"
)

// ui.Client
// -> UI: ui.UI
//    -> UI.HLClient

func NewUIClient(cfgPath string, logger *slog.Logger) *Client {
	c := &Client{
		CfgPath: cfgPath,
		Logger:  logger,
		//activeTasks: make(map[uint32]*Transaction),
		//Handlers: DefaultClientHandlers,q
	}

	prefs, err := readConfig(cfgPath)
	if err != nil {
		logger.Error(fmt.Sprintf("unable to read config file %s\n", cfgPath))
		os.Exit(1)
	}
	c.Pref = prefs
	c.UI = NewUI(c)

	return c
}

func readConfig(cfgPath string) (*ClientPrefs, error) {
	fh, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}

	prefs := ClientPrefs{}
	decoder := yaml.NewDecoder(fh)
	if err := decoder.Decode(&prefs); err != nil {
		return nil, err
	}
	return &prefs, nil
}

func NewUI(c *Client) *UI {
	app := tview.NewApplication()
	chatBox := tview.NewTextView().
		SetScrollable(true).
		SetDynamicColors(true).
		SetWordWrap(true).
		SetChangedFunc(func() {
			app.Draw() // TODO: docs say this is bad but it's the only way to show content during initial render??
		})
	chatBox.Box.SetBorder(true).SetTitle("| Chat |")

	chatInput := tview.NewInputField()
	chatInput.
		SetLabel("> ").
		SetFieldBackgroundColor(tcell.ColorDimGray).
		SetDoneFunc(func(key tcell.Key) {
			// skip send if user hit enter with no other text
			if len(chatInput.GetText()) == 0 {
				return
			}

			_ = c.UI.HLClient.Send(
				*hotline.NewTransaction(hotline.TranChatSend, nil,
					hotline.NewField(hotline.FieldData, []byte(chatInput.GetText())),
				),
			)
			chatInput.SetText("") // clear the input field after chat send
		})

	chatInput.Box.SetBorder(true).SetTitle("Send")

	userList := tview.
		NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() {
			app.Draw() // TODO: docs say this is bad but it's the only way to show content during initial render??
		})
	userList.Box.SetBorder(true).SetTitle("Users")

	return &UI{
		App:         app,
		chatBox:     chatBox,
		Pages:       tview.NewPages(),
		chatInput:   chatInput,
		userList:    userList,
		trackerList: tview.NewList(),
		HLClient:    hotline.NewClient(c.Pref.Username, c.Logger),
		Pref:        c.Pref,
	}
}

func (ui *UI) showBookmarks() *tview.List {
	list := tview.NewList()
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			ui.Pages.SwitchToPage("home")
		}
		return event
	})
	list.Box.SetBorder(true).SetTitle("| Bookmarks |")

	shortcut := 97 // rune for "a"
	for i, srv := range ui.Pref.Bookmarks {
		addr := srv.Addr
		login := srv.Login
		pass := srv.Password
		list.AddItem(srv.Name, srv.Addr, rune(shortcut+i), func() {
			ui.Pages.RemovePage("joinServer")

			newJS := ui.renderJoinServerForm("", addr, login, pass, "bookmarks", true, true)

			ui.Pages.AddPage("joinServer", newJS, true, true)
		})
	}

	return list
}

func (ui *UI) getTrackerList(servers []hotline.ServerRecord) *tview.List {
	list := tview.NewList()
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			ui.Pages.SwitchToPage("home")
		}
		return event
	})
	list.Box.SetBorder(true).SetTitle("| Servers |")

	const shortcut = 97 // rune for "a"
	for i, _ := range servers {
		srv := servers[i]
		list.AddItem(string(srv.Name), string(srv.Description), rune(shortcut+i), func() {
			ui.Pages.RemovePage("joinServer")

			newJS := ui.renderJoinServerForm(string(srv.Name), srv.Addr(), hotline.GuestAccount, "", trackerListPage, false, true)

			ui.Pages.AddPage("joinServer", newJS, true, true)
			ui.Pages.ShowPage("joinServer")
		})
	}

	return list
}

func (ui *UI) renderSettingsForm() *tview.Flex {
	iconStr := strconv.Itoa(ui.Pref.IconID)
	settingsForm := tview.NewForm()
	settingsForm.AddInputField("Your Name", ui.Pref.Username, 0, nil, nil)
	settingsForm.AddInputField("IconID", iconStr, 0, func(idStr string, _ rune) bool {
		_, err := strconv.Atoi(idStr)
		return err == nil
	}, nil)
	settingsForm.AddInputField("Tracker", ui.Pref.Tracker, 0, nil, nil)
	settingsForm.AddCheckbox("Enable Terminal Bell", ui.Pref.EnableBell, nil)
	settingsForm.AddButton("Save", func() {
		usernameInput := settingsForm.GetFormItem(0).(*tview.InputField).GetText()
		if len(usernameInput) == 0 {
			usernameInput = "unnamed"
		}
		ui.Pref.Username = usernameInput
		iconStr = settingsForm.GetFormItem(1).(*tview.InputField).GetText()
		ui.Pref.IconID, _ = strconv.Atoi(iconStr)
		ui.Pref.Tracker = settingsForm.GetFormItem(2).(*tview.InputField).GetText()
		ui.Pref.EnableBell = settingsForm.GetFormItem(3).(*tview.Checkbox).IsChecked()

		out, err := yaml.Marshal(&ui.Pref)
		if err != nil {
			// TODO: handle err
		}
		// TODO: handle err
		err = os.WriteFile(ui.HLClient.CfgPath, out, 0666)
		if err != nil {
			println(ui.HLClient.CfgPath)
			panic(err)
		}
		ui.Pages.RemovePage("settings")
	})
	settingsForm.SetBorder(true)
	settingsForm.SetCancelFunc(func() {
		ui.Pages.RemovePage("settings")
	})
	settingsPage := tview.NewFlex().SetDirection(tview.FlexRow)
	settingsPage.Box.SetBorder(true).SetTitle("Settings")
	settingsPage.AddItem(settingsForm, 0, 1, true)

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(settingsForm, 15, 1, true).
			AddItem(nil, 0, 1, false), 40, 1, true).
		AddItem(nil, 0, 1, false)

	return centerFlex
}

func (ui *UI) joinServer(addr, login, password string) error {
	// append default port to address if no port supplied
	if len(strings.Split(addr, ":")) == 1 {
		addr += ":5500"
	}
	if err := ui.HLClient.Connect(addr, login, password); err != nil {
		return fmt.Errorf("Error joining server: %v\n", err)
	}

	go func() {
		if err := ui.HLClient.HandleTransactions(context.TODO()); err != nil {
			ui.Pages.SwitchToPage("home")
		}

		loginErrModal := tview.NewModal().
			AddButtons([]string{"Ok"}).
			SetText("The server connection has closed.").
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				ui.Pages.SwitchToPage("home")
			})
		loginErrModal.Box.SetTitle("Server Connection Error")

		ui.Pages.AddPage("loginErr", loginErrModal, false, true)
		ui.App.Draw()
	}()

	return nil
}

func (ui *UI) renderJoinServerForm(name, server, login, password, backPage string, save, defaultConnect bool) *tview.Flex {
	joinServerForm := tview.NewForm()
	joinServerForm.
		AddInputField("Server", server, 0, nil, nil).
		AddInputField("Login", login, 0, nil, nil).
		AddPasswordField("Password", password, 0, '*', nil).
		AddCheckbox("Save", save, func(checked bool) {
			ui.Pref.AddBookmark(
				joinServerForm.GetFormItem(0).(*tview.InputField).GetText(),
				joinServerForm.GetFormItem(0).(*tview.InputField).GetText(),
				joinServerForm.GetFormItem(1).(*tview.InputField).GetText(),
				joinServerForm.GetFormItem(2).(*tview.InputField).GetText(),
			)

			out, err := yaml.Marshal(ui.Pref)
			if err != nil {
				panic(err)
			}

			err = os.WriteFile(ui.HLClient.CfgPath, out, 0666)
			if err != nil {
				panic(err)
			}
		}).
		AddButton("Cancel", func() {
			ui.Pages.SwitchToPage(backPage)
		}).
		AddButton("Connect", func() {
			srvAddr := joinServerForm.GetFormItem(0).(*tview.InputField).GetText()
			loginInput := joinServerForm.GetFormItem(1).(*tview.InputField).GetText()
			err := ui.joinServer(
				srvAddr,
				loginInput,
				joinServerForm.GetFormItem(2).(*tview.InputField).GetText(),
			)
			if name == "" {
				name = fmt.Sprintf("%s@%s", loginInput, srvAddr)
			}
			ui.HLClient.ServerName = name

			if err != nil {
				ui.HLClient.Logger.Error("login error", "err", err)
				loginErrModal := tview.NewModal().
					AddButtons([]string{"Oh no"}).
					SetText(err.Error()).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						ui.Pages.SwitchToPage(backPage)
					})

				ui.Pages.AddPage("loginErr", loginErrModal, false, true)
			}

			// Save checkbox
			if joinServerForm.GetFormItem(3).(*tview.Checkbox).IsChecked() {
				// TODO: implement bookmark saving
			}
		})

	joinServerForm.Box.SetBorder(true).SetTitle("| Connect |")
	joinServerForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.Pages.SwitchToPage(backPage)
		}
		return event
	})

	if defaultConnect {
		joinServerForm.SetFocus(5)
	}

	joinServerPage := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(joinServerForm, 14, 1, true).
			AddItem(nil, 0, 1, false), 40, 1, true).
		AddItem(nil, 0, 1, false)

	return joinServerPage
}

func (ui *UI) renderServerUI() *tview.Flex {
	ui.chatBox.SetText("") // clear any previously existing chatbox text
	commandList := tview.NewTextView().SetDynamicColors(true)
	commandList.
		SetText("[yellow]^n[-::]: Read News   [yellow]^p[-::]: Post News\n[yellow]^l[-::]: View Logs   [yellow]^f[-::]: View Files\n").
		SetBorder(true).
		SetTitle("| Keyboard Shortcuts| ")

	modal := tview.NewModal().
		SetText("Disconnect from the server?").
		AddButtons([]string{"Cancel", "Exit"}).
		SetFocus(1)
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonIndex == 1 {
			_ = ui.HLClient.Disconnect()
			ui.Pages.RemovePage(pageServerUI)
			ui.Pages.SwitchToPage("home")
		} else {
			ui.Pages.HidePage("modal")
		}
	})

	serverUI := tview.NewFlex().
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(commandList, 4, 0, false).
			AddItem(ui.chatBox, 0, 8, false).
			AddItem(ui.chatInput, 3, 0, true), 0, 1, true).
		AddItem(ui.userList, 25, 1, false)
	serverUI.SetBorder(true).SetTitle("| Mobius - Connected to " + ui.HLClient.ServerName + " |").SetTitleAlign(tview.AlignLeft)
	serverUI.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.Pages.AddPage("modal", modal, false, true)
		}

		// List files
		if event.Key() == tcell.KeyCtrlF {
			if err := ui.HLClient.Send(*hotline.NewTransaction(hotline.TranGetFileNameList, nil)); err != nil {
				ui.HLClient.Logger.Error("err", "err", err)
			}
		}

		// Show News
		if event.Key() == tcell.KeyCtrlN {
			if err := ui.HLClient.Send(*hotline.NewTransaction(hotline.TranGetMsgs, nil)); err != nil {
				ui.HLClient.Logger.Error("err", "err", err)
			}
		}

		// Post news
		if event.Key() == tcell.KeyCtrlP {
			newsFlex := tview.NewFlex()
			newsFlex.SetBorderPadding(0, 0, 1, 1)
			newsPostTextArea := tview.NewTextView()
			newsPostTextArea.SetBackgroundColor(tcell.ColorDarkSlateGrey)
			newsPostTextArea.SetChangedFunc(func() {
				ui.App.Draw() // TODO: docs say this is bad but it's the only way to show content during initial render??
			})

			newsPostForm := tview.NewForm().
				SetButtonsAlign(tview.AlignRight).
				// AddButton("Cancel", nil). // TODO: implement cancel button behavior
				AddButton("Send", nil)
			newsPostForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyEscape:
					ui.Pages.RemovePage("newsInput")
				case tcell.KeyTab:
					ui.App.SetFocus(newsPostTextArea)
				case tcell.KeyEnter:
					newsText := strings.ReplaceAll(newsPostTextArea.GetText(true), "\n", "\r")
					if len(newsText) == 0 {
						return event
					}
					err := ui.HLClient.Send(
						*hotline.NewTransaction(hotline.TranOldPostNews, nil,
							hotline.NewField(hotline.FieldData, []byte(newsText)),
						),
					)
					if err != nil {
						ui.HLClient.Logger.Error("Error posting news", "err", err)
						// TODO: display errModal to user
					}
					ui.Pages.RemovePage("newsInput")
				}

				return event
			})

			newsFlex.
				SetDirection(tview.FlexRow).
				SetBorder(true).
				SetTitle("| Post Message |")

			newsPostTextArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				switch event.Key() {
				case tcell.KeyEscape:
					ui.Pages.RemovePage("newsInput")
				case tcell.KeyTab:
					ui.App.SetFocus(newsPostForm)
				case tcell.KeyEnter:
					_, _ = fmt.Fprintf(newsPostTextArea, "\n")
				default:
					const windowsBackspaceRune = 8
					const macBackspaceRune = 127
					switch event.Rune() {
					case macBackspaceRune, windowsBackspaceRune:
						curTxt := newsPostTextArea.GetText(true)
						if len(curTxt) > 0 {
							curTxt = curTxt[:len(curTxt)-1]
							newsPostTextArea.SetText(curTxt)
						}
					default:
						_, _ = fmt.Fprint(newsPostTextArea, string(event.Rune()))
					}
				}

				return event
			})

			newsFlex.AddItem(newsPostTextArea, 10, 0, true)
			newsFlex.AddItem(newsPostForm, 3, 0, false)

			newsPostPage := tview.NewFlex().
				AddItem(nil, 0, 1, false).
				AddItem(tview.NewFlex().
					SetDirection(tview.FlexRow).
					AddItem(nil, 0, 1, false).
					AddItem(newsFlex, 15, 1, true).
					// AddItem(newsPostForm, 3, 0, false).
					AddItem(nil, 0, 1, false), 40, 1, false).
				AddItem(nil, 0, 1, false)

			ui.Pages.AddPage("newsInput", newsPostPage, true, true)
			ui.App.SetFocus(newsPostTextArea)
		}

		return event
	})
	return serverUI
}

func (ui *UI) Start() {
	home := tview.NewFlex().SetDirection(tview.FlexRow)
	home.Box.SetBorder(true).SetTitle("| Mobius v" + hotline.VERSION + " |").SetTitleAlign(tview.AlignLeft)
	mainMenu := tview.NewList()

	bannerItem := tview.NewTextView().
		SetText(randomBanner()).
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	home.AddItem(
		tview.NewFlex().AddItem(bannerItem, 0, 1, false),
		14, 1, false)
	home.AddItem(tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(mainMenu, 0, 1, true).
		AddItem(nil, 0, 1, false),
		0, 1, true,
	)

	mainMenu.AddItem("Join Server", "", 'j', func() {
		joinServerPage := ui.renderJoinServerForm("", "", hotline.GuestAccount, "", "home", false, false)
		ui.Pages.AddPage("joinServer", joinServerPage, true, true)
	}).
		AddItem("Bookmarks", "", 'b', func() {
			ui.Pages.AddAndSwitchToPage("bookmarks", ui.showBookmarks(), true)
		}).
		AddItem("Browse Tracker", "", 't', func() {
			listing, err := hotline.GetListing(ui.Pref.Tracker)
			if err != nil {
				errMsg := fmt.Sprintf("Error fetching tracker results:\n%v", err)
				errModal := tview.NewModal()
				errModal.SetText(errMsg)
				errModal.AddButtons([]string{"Cancel"})
				errModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					ui.Pages.RemovePage("errModal")
				})
				ui.Pages.RemovePage("joinServer")
				ui.Pages.AddPage("errModal", errModal, false, true)
				return
			}
			ui.trackerList = ui.getTrackerList(listing)
			ui.Pages.AddAndSwitchToPage("trackerList", ui.trackerList, true)
		}).
		AddItem("Settings", "", 's', func() {
			ui.Pages.AddPage("settings", ui.renderSettingsForm(), true, true)
		}).
		AddItem("Quit", "", 'q', func() {
			ui.App.Stop()
		})

	ui.Pages.AddPage("home", home, true, true)

	// App level input capture
	ui.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			ui.HLClient.Logger.Info("Exiting")
			ui.App.Stop()
			os.Exit(0)
		}
		// Show Logs
		if event.Key() == tcell.KeyCtrlL {
			ui.HLClient.DebugBuf.TextView.ScrollToEnd()
			ui.HLClient.DebugBuf.TextView.SetBorder(true).SetTitle("Logs")
			ui.HLClient.DebugBuf.TextView.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEscape {
					ui.Pages.RemovePage("logs")
				}
			})

			ui.Pages.AddPage("logs", ui.HLClient.DebugBuf.TextView, true, true)
		}
		return event
	})

	if err := ui.App.SetRoot(ui.Pages, true).SetFocus(ui.Pages).Run(); err != nil {
		ui.App.Stop()
		os.Exit(1)
	}
}
