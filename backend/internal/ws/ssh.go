package ws

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"cloudmanager/internal/logging"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// WebSocket / SSH 消息类型（JSON 文本协议）
const (
	MsgAuth   = "auth"   // 客户端 -> 服务端：SSH 登录凭据
	MsgData   = "data"   // 双向：终端数据流
	MsgResize = "resize" // 客户端 -> 服务端：窗口尺寸变更
	MsgStatus = "status" // 服务端 -> 客户端：连接状态 / 错误
	MsgExit   = "exit"   // 服务端 -> 客户端：会话退出
)

// WSMsg 单向文本消息。同一结构用于双端，按 Type 区分含义。
type WSMsg struct {
	Type       string `json:"type"`
	Data       string `json:"data,omitempty"`       // 终端输出/输入
	Username   string `json:"username,omitempty"`   // auth
	Password   string `json:"password,omitempty"`   // auth（密码方式）
	PrivateKey string `json:"private_key,omitempty"` // auth（密钥方式，PEM 文本）
	Passphrase string `json:"passphrase,omitempty"`  // 私钥口令（可选）
	Port       int    `json:"port,omitempty"`       // auth：SSH 端口（默认 22）
	Cols       int    `json:"cols,omitempty"`        // resize
	Rows       int    `json:"rows,omitempty"`        // resize
	Status     string `json:"status,omitempty"`      // status: connected / error
	Message    string `json:"message,omitempty"`     // status / exit 描述
	Code       int    `json:"code,omitempty"`        // exit 码
}

// SSHOptions 目标服务器连接参数
type SSHOptions struct {
	Host    string        // 公网 IP
	Port    int           // 默认 22
	Timeout time.Duration // 拨号超时
}

var upgrader = websocket.Upgrader{
	// 跨域由 JWT 在 handler 层校验，这里放开 Origin 检查
	CheckOrigin: func(r *http.Request) bool { return true },
	Subprotocols: []string{"webssh"},
}

// HandleWebShell 处理单个 WebShell WebSocket 连接：校验凭据后建立 SSH 会话，
// 并双向桥接 WebSocket <-> SSH 会话。返回值表示是否成功建立 SSH 连接（用于审计）。
func HandleWebShell(w http.ResponseWriter, r *http.Request, opts SSHOptions) bool {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return false
	}
	defer conn.Close()

	// 1) 读取首条 auth 消息获取 SSH 凭据（带读超时，避免空连接挂起）
	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	_, buf, err := conn.ReadMessage()
	if err != nil {
		return false
	}
	var auth WSMsg
	if err := json.Unmarshal(buf, &auth); err != nil || auth.Type != MsgAuth {
		writeMsg(conn, WSMsg{Type: MsgStatus, Status: "error", Message: "缺少 SSH 登录凭据"})
		return false
	}
	conn.SetReadDeadline(time.Time{})

	// 2) 建立 SSH 连接（auth.port 可覆盖默认 22，用于非标端口）
	if auth.Port > 0 {
		opts.Port = auth.Port
	}
	client, session, err := dialSSH(opts, auth)
	if err != nil {
		writeMsg(conn, WSMsg{Type: MsgStatus, Status: "error", Message: err.Error()})
		return false
	}
	defer client.Close()

	// 3) 分配 PTY + 启动登录 shell
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		writeMsg(conn, WSMsg{Type: MsgStatus, Status: "error", Message: "请求 PTY 失败: " + err.Error()})
		return false
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		writeMsg(conn, WSMsg{Type: MsgStatus, Status: "error", Message: err.Error()})
		return false
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		writeMsg(conn, WSMsg{Type: MsgStatus, Status: "error", Message: err.Error()})
		return false
	}
	if err := session.Shell(); err != nil {
		writeMsg(conn, WSMsg{Type: MsgStatus, Status: "error", Message: err.Error()})
		return false
	}
	writeMsg(conn, WSMsg{Type: MsgStatus, Status: "connected", Message: fmt.Sprintf("已连接到 %s", opts.Host)})

	// 4) 双向桥接。done 由任一方关闭触发，统一收尾
	var once sync.Once
	done := make(chan struct{})
	closeAll := func() { once.Do(func() { close(done) }) }

	// SSH stdout/stderr -> WebSocket
	go func() {
		defer logging.Recover("ssh-stdout-to-ws")
		buf := make([]byte, 32*1024)
		for {
			n, rerr := stdout.Read(buf)
			if n > 0 {
				writeMsg(conn, WSMsg{Type: MsgData, Data: string(buf[:n])})
			}
			if rerr != nil {
				closeAll()
				return
			}
		}
	}()

	// WebSocket -> SSH stdin（含窗口尺寸变更）
	go func() {
		defer logging.Recover("ws-to-ssh-stdin")
		for {
			_, msg, rerr := conn.ReadMessage()
			if rerr != nil {
				closeAll()
				return
			}
			var m WSMsg
			if jsonErr := json.Unmarshal(msg, &m); jsonErr != nil {
				continue
			}
			switch m.Type {
			case MsgData:
				io.WriteString(stdin, m.Data)
			case MsgResize:
				_ = session.WindowChange(m.Rows, m.Cols)
			}
		}
	}()

	// SSH 会话结束（shell 退出 / 连接断开）后统一收尾
	go func() {
		defer logging.Recover("ssh-session-wait")
		_ = session.Wait()
		closeAll()
	}()

	<-done
	writeMsg(conn, WSMsg{Type: MsgExit, Code: 0})
	return true
}

// dialSSH 按凭据建立 SSH 客户端与会话。
func dialSSH(opts SSHOptions, auth WSMsg) (*ssh.Client, *ssh.Session, error) {
	if auth.Username == "" {
		return nil, nil, fmt.Errorf("请提供 SSH 登录用户名")
	}
	config := &ssh.ClientConfig{
		User:            auth.Username,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 管理终端场景，接受任意主机密钥
		Timeout:         opts.Timeout,
	}
	if auth.PrivateKey != "" {
		var signer ssh.Signer
		var keyErr error
		if auth.Passphrase != "" {
			signer, keyErr = ssh.ParsePrivateKeyWithPassphrase([]byte(auth.PrivateKey), []byte(auth.Passphrase))
		} else {
			signer, keyErr = ssh.ParsePrivateKey([]byte(auth.PrivateKey))
		}
		if keyErr != nil {
			return nil, nil, fmt.Errorf("私钥解析失败: %w", keyErr)
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}
	if auth.Password != "" {
		config.Auth = append(config.Auth, ssh.Password(auth.Password))
	}
	if len(config.Auth) == 0 {
		return nil, nil, fmt.Errorf("请提供密码或私钥")
	}

	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, nil, fmt.Errorf("SSH 连接失败: %w", err)
	}
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("创建会话失败: %w", err)
	}
	return client, session, nil
}

func writeMsg(conn *websocket.Conn, m WSMsg) {
	_ = conn.WriteMessage(websocket.TextMessage, mustJSON(m))
}

func mustJSON(m WSMsg) []byte {
	b, _ := json.Marshal(m)
	return b
}
