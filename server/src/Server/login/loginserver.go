package login

import (
	"Server/cluster"
	"fmt"
	"gameproto/msgs"
	"github.com/magicsea/ganet/config"
	"github.com/magicsea/ganet/log"
	"github.com/magicsea/ganet/service"
	"net/http"

	"Server/db"
	"gameproto"

	"strconv"

	"time"

	"github.com/magicsea/ganet/network"
	//"Server/config"
)

type LoginService struct {
	service.ServiceData
}

//Service 获取服务对象
func Service() service.IService {
	return new(LoginService)
}

func Type() string {
	return "login"
}

//以下为接口函数
func (s *LoginService) OnReceive(context service.Context) {
	fmt.Println("center.OnReceive:", context.Message())
}
func (s *LoginService) OnInit() {

}

func (s *LoginService) OnStart(as *service.ActorService) {
	//as.RegisterMsg(reflect.TypeOf(&messages.UserLogin{}), s.OnUserLogin) //注册登录

	//开启rpc,任意端口
	//remote.Start("127.0.0.1:0")
	//cluster.Start(&cluster.ClusterConfig{"127.0.0.1:8090", "127.0.0.1:8091"})

	go func() {
		//开启http服务
		http.HandleFunc("/login", login)
		http.HandleFunc("/regist", regist)
		httpAddr := config.GetServiceConfigString(s.Name, "httpAddr")
		log.Info("login listen http:", s.Name, "  ", httpAddr)
		http.ListenAndServe(httpAddr, nil)
	}()

}

//注册
func regist(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	req.ParseForm()
	if req.Form["a"] == nil || req.Form["p"] == nil {
		log.Error("a,p is empty:", req.Form)
		return
	}
	//账号
	acc := ""
	if al, ok := req.Form["a"]; ok {
		acc = al[0]
	}
	//密码
	pwd := ""
	if al, ok := req.Form["p"]; ok {
		pwd = al[0]
	}

	if len(acc) < 1 || len(pwd) < 1 {
		registBackError(w, "账号密码都不能为空", nil)
		return
	}

	log.Info("reg account:acc=%s,pwd=%s", acc, pwd)

	key := "User:nameindex:" + acc
	r := db.GetRedisGame().Get(key).Val()
	// if err1!=nil {
	// 	registBackError(w,"数据插入,获取索引出错",err1)
	// 	return;
	// }
	if len(r) > 0 {
		registBackError(w, "已经存在的账号", nil)
		return
	}

	//插入
	gamedb := db.GetRedisGame()
	id, err2 := gamedb.Incr("User:Id").Result()
	if err2 != nil {
		registBackError(w, "数据插入id出错", err2)
		return
	}

	var user = &db.User{Id: id, Account: acc, Password: pwd, RegisterTime: time.Now().Unix()}
	if err := db.SetRedisObject(user, id, gamedb); err != nil {
		registBackError(w, "数据插入出错", err)
		return
	}

	//设置索引
	db.GetRedisGame().Set(key, id, 0)

	w.Write([]byte("success"))
}

func registBackError(w http.ResponseWriter, val string, e error) {
	log.Error("create user db error:%s,%v", val, e)
	w.Write([]byte(val))
}

//登录
func login(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	req.ParseForm()
	if req.Form["a"] == nil || req.Form["p"] == nil {
		log.Error("a,p is empty:", req.Form)
		return
	}
	//账号
	acc := ""
	if al, ok := req.Form["a"]; ok {
		acc = al[0]
	}

	pwd := ""
	if pl, ok := req.Form["p"]; ok {
		pwd = pl[0]
	}

	//协议，默认pb，否则json
	// proto:="pb"
	// if al, ok := req.Form["proto"]; ok {
	// 	proto = al[0]
	// }

	//验证 here...
	log.Info("login account:acc=%s,pwd=%s", acc, pwd)
	gamedb := db.GetRedisGame()
	//索引
	key := "User:nameindex:" + acc
	r, err := db.GetRedisGame().Get(key).Result()
	if err != nil {
		loginBackError(w, "get username error:"+key, err)
		return
	}
	if len(r) < 1 {
		loginBackError(w, "username not exist:"+key, nil)
		return
	}

	//账号密码
	now := time.Now().Unix()
	user := &db.User{}
	found, e := db.GetRedisObject(user, r, gamedb)
	if e != nil && !found {
		loginBackError(w, "not found user:"+r, e)
		return
	}
	if user.Password != pwd {
		loginBackError(w, "password error:"+user.Password+"!="+pwd, nil)
		return
	}

	//保存
	db.SetRedisObjectField(user, r, gamedb, "LastLoginTime", now)
	id, _ := strconv.Atoi(r)
	resp, err := cluster.GetServicePID("session").Ask(&msgs.UserLogin{acc, uint64(id)})
	if err == nil {
		var s, _ = network.Marshal(resp.(*gameproto.UserLoginResult))
		w.Write(s)
		log.Info("login ok:msg=%v", resp)
	} else {
		loginBackError(w, "ask session error", err)
		log.Info("login error:", acc, err)
	}
}

func loginBackError(w http.ResponseWriter, info string, e error) {
	log.Error("login user db fail:%v,%v", info, e)
	var m = &gameproto.UserLoginResult{Result: int32(msgs.Error)}
	d, _ := network.Marshal(m)
	w.Write(d)
}
