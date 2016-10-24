package umeng

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	Platform_Android = iota
	Platform_iOS
)

const (
	PushType_Unicast = iota
	PushType_Listcast
	PushType_Filecast
	PushType_Broadcast
	PushType_Groupcast
	PushType_Customizedcast
)

const (
	MsgType_Notification = iota
	MsgType_Message
)

const (
	pushMessageUrl    = "http://msg.umeng.com/api/send"
	pushTaskStatusUrl = "http://msg.umeng.com/api/status"
	pushTaskCacelUrl  = " http://msg.umeng.com/api/cancel"
	pushFileUploadUrl = "http://msg.umeng.com/upload"
)

type UPush struct {
	pushtype int
	msgtype  int
	appkey   string
	secret   string
	policy   map[string]interface{}
	extra    map[string]string
	apns     map[string]interface{}
	body     map[string]interface{}
	message  map[string]interface{}
}

func NewPush(pushtype int, msgtype int, appkey, master_secret string) *UPush {
	push := &UPush{
		pushtype: pushtype,
		msgtype:  msgtype,
		appkey:   appkey,
		secret:   master_secret,
		policy:   nil,
		extra:    nil,
		apns:     nil,
		body:     nil,
		message:  make(map[string]interface{}),
	}

	push.message["appkey"] = appkey
	switch pushtype {
	case PushType_Unicast:
		push.message["type"] = "unicast"
	case PushType_Listcast:
		push.message["type"] = "listcast"
	case PushType_Filecast:
		push.message["type"] = "filecast"
	case PushType_Broadcast:
		push.message["type"] = "broadcast"
	case PushType_Groupcast:
		push.message["type"] = "groupcast"
	case PushType_Customizedcast:
		push.message["type"] = "customizedcast"
	default:
		push.pushtype = PushType_Broadcast
		push.message["type"] = "broadcast"
	}

	return push
}

func (p *UPush) Token(token string) *UPush {
	if p.pushtype != PushType_Unicast && p.pushtype != PushType_Listcast {
		return p
	}

	tokens := strings.Split(token, ",")
	size := len(tokens)
	if size == 0 {
		return p
	}

	if p.pushtype == PushType_Unicast {
		p.message["device_tokens"] = tokens[0]
	} else if p.pushtype == PushType_Listcast {
		if size > 500 {
			log.Println("Device tokens is more than 500, and only 500 tokens will be used.")
			tmpTokens := tokens[:500]
			strTokens := strings.Join(tmpTokens, ",")
			p.message["device_tokens"] = strTokens
		} else {
			p.message["devicd_tokens"] = token
		}
	}

	return p
}

func (p *UPush) Alias(typ, alias string) *UPush {
	if p.pushtype != PushType_Customizedcast {
		return p
	}

	p.message["alias_type"] = typ

	aliases := strings.Split(alias, ",")
	if len(aliases) > 50 {
		log.Println("Alias is more than 50, and only 50 alias will be used.")
		tmpAliases := aliases[:50]
		p.message["alias"] = strings.Join(tmpAliases, ",")
	} else {
		p.message["alias"] = alias
	}

	return p
}

func (p *UPush) FileId(id string) *UPush {

	if p.pushtype != PushType_Filecast {
		return p
	}

	p.message["file_id"] = id
	return p
}

func (p *UPush) Filter(filter string) *UPush {
	return p
}

func (p *UPush) Extra(key, value string) *UPush {

	if p.extra == nil {
		p.extra = make(map[string]string)
	}

	p.extra[key] = value

	return p
}

func (p *UPush) Body(key string, value interface{}) *UPush {

	if p.msgtype == MsgType_Message && key != "custom" {
		return p
	}

	if p.body == nil {
		p.body = make(map[string]interface{})
	}

	p.body[key] = value

	return p
}

func (p *UPush) APNs(key string, value interface{}) *UPush {

	if p.apns == nil {
		p.apns = make(map[string]interface{})
	}

	p.apns[key] = value

	return p
}

func (p *UPush) Policy(key string, value interface{}) *UPush {

	if p.policy == nil {
		p.policy = make(map[string]interface{})
	}

	p.policy[key] = value

	return p
}

func (p *UPush) Mode(production bool) *UPush {

	if production {
		p.message["production_mode"] = "true"
	} else {
		p.message["production_mode"] = "false"
	}

	return p
}

func (p *UPush) Description(desc string) *UPush {
	p.message["description"] = desc

	return p
}

func (p *UPush) ThirdpartyId(id string) *UPush {
	p.message["thirdparty_id"] = id

	return p
}

func (p *UPush) Push(platform int) (result string, err error) {

	timestamp := time.Now().Unix()
	p.message["timestamp"] = fmt.Sprintf("%v", timestamp)
	payload := map[string]interface{}{}
	if platform == Platform_Android {
		if p.msgtype == MsgType_Notification {
			payload["display_type"] = "notification"
			payload["body"] = p.body
		} else {
			payload["display_type"] = "message"
			payload["body"] = p.body
			if len(p.extra) > 0 {
				payload["extra"] = p.extra
			}
		}
	} else {
		payload["aps"] = p.apns
		for key, value := range p.extra {
			payload[key] = value
		}
	}

	p.message["payload"] = payload
	policy := make(map[string]interface{})
	for key, value := range p.policy {
		if platform == Platform_iOS && key == "out_biz_no" {
			continue
		}

		policy[key] = value
	}
	p.message["policy"] = policy

	body, err := json.Marshal(p.message)
	if err != nil {
		return
	}

	signature := p.sign("POST", pushMessageUrl, string(body))

	fmt.Println(signature)
	fmt.Println(string(body))

	url := pushMessageUrl + "?sign=" + signature

	var resp *http.Response
	resp, err = http.Post(url, "application/json;charset=utf-8", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (p *UPush) Status(taskId string) (result string, err error) {
	return p.post(pushTaskStatusUrl, "task_id", taskId)
}

func (p *UPush) Cancel(taskId string) (result string, err error) {
	return p.post(pushTaskCacelUrl, "task_id", taskId)
}

func (p *UPush) Upload(content string) (result string, err error) {
	return p.post(pushFileUploadUrl, "content", content)
}

func (p *UPush) post(url, key, value string) (result string, err error) {
	timestamp := time.Now().Unix()
	msg := map[string]string{"appkey": p.appkey, key: value, "timestamp": fmt.Sprintf("%v", timestamp)}
	body, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}

	signature := p.sign("POST", url, string(body))

	uri := url + "?sign=" + signature

	resp, err := http.Post(uri, "application/json;charset=utf-8", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func (p *UPush) sign(method, url, message string) string {
	signstr := method + url + message + p.secret
	hash := md5.New()
	io.WriteString(hash, signstr)
	return fmt.Sprintf("%02x", hash.Sum(nil))
}
