package transport

import (
	"bytes"
	"encoding/gob"
	"errors"
	"flag"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gitferry/bamboo/log"
)

var Scheme = flag.String("transport", "tcp", "transport scheme (tcp, udp, chan), default tcp")

// Transport = transport + pipe + client + server
type Transport interface {
	// Scheme returns tranport scheme
	Scheme() string

	// Send sends message into t.send chan
	Send(interface{})

	// Recv waits for message from t.recv chan
	Recv() interface{}

	// Dial connects to remote server non-blocking once connected
	Dial() error

	// Listen waits for connections, non-blocking once listener starts
	Listen()

	SendBitsCount() int64

	RecvBitsCount() int64

	// Close closes send channel and stops listener
	Close()

	GetUrl() string
}

// NewTransport creates new transport object with url
func NewTransport(addr string, ch chan struct{}) Transport {
	if !strings.Contains(addr, "://") {
		addr = *Scheme + "://" + addr
	}
	uri, err := url.Parse(addr)
	if err != nil {
		log.Fatalf("error parsing address %s : %s\n", addr, err)
	}

	transport := &transport{
		url:             uri,
		send:            make(chan interface{}, 102400),
		recv:            make(chan interface{}, 102400),
		close:           make(chan struct{}),
		concurrentLimit: ch,
	}

	switch uri.Scheme {
	case "chan":
		t := new(channel)
		t.transport = transport
		return t
	case "tcp":
		t := new(tcp)
		t.transport = transport
		return t
	case "udp":
		t := new(udp)
		t.transport = transport
		return t
	default:
		log.Fatalf("unknown scheme %s", uri.Scheme)
	}
	return nil
}

type transport struct {
	url              *url.URL
	send             chan interface{}
	recv             chan interface{}
	startSendingTime time.Time
	startRecvTime    time.Time
	totalSentBits    int64
	totalRecvBits    int64
	close            chan struct{}
	concurrentLimit  chan struct{}
}

func (t *transport) GetUrl() string {
	return t.url.Host
}

func (t *transport) Send(m interface{}) {
	t.send <- m
}

func (t *transport) Recv() interface{} {
	return <-t.recv
}

func (t *transport) Close() {
	close(t.send)
	close(t.close)
}

func (t *transport) Scheme() string {
	return t.url.Scheme
}

func (t *transport) Dial() error {
	conn, err := net.Dial(t.Scheme(), t.url.Host)
	if err != nil {
		return err
	}
	t.startSendingTime = time.Now()

	go func(conn net.Conn) {
		// w := bufio.NewWriter(conn)
		// codec := NewCodec(config.Codec, conn)
		encoder := gob.NewEncoder(conn)
		// 使用远程地址作为协程标识符
		connName := conn.RemoteAddr().String()
		var num int32 = 0

		defer conn.Close()
		log.Debugf("Dial() --- 已连接，等待数据")
		for m := range t.send {
			log.Debugf("Dial() --- 收到数据，启动一个协程来发送")

			// 确保释放并发限制

			log.Debugf("Dial() --- 准备发送到[%v]，发送数据为%T,t.send的当前待发送的数据有%v条", connName, m, len(t.send))
			err := encoder.Encode(&m)
			if err != nil {
				log.Errorf("Dial() --- 发送到[%v]，发送数据为%T err:[%v]", connName, m, err)
			}
			<-t.concurrentLimit
			log.Debugf("Dial() --- 发送到[%v]成功, 限流器大小[%v] 发送数据为%T,准备写入本地缓冲区来获取发送的数据量 No.[%v]", connName, len(t.concurrentLimit), m, num)
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			enc.Encode(&m)
			t.totalSentBits += int64(buf.Len()) * 8
			log.Debugf("Dial() --- 发送到[%v]的数据量,获取成功， 发送数据为%T,发送数据量为%v No.[%v]", connName, m, buf.Len(), num)
			num++
		}
	}(conn)

	return nil
}

func (t *transport) SendBitsCount() int64 {
	rate := int64(float64(t.totalSentBits) / time.Now().Sub(t.startSendingTime).Seconds())
	t.totalSentBits = 0
	t.startSendingTime = time.Now()
	return rate
}

func (t *transport) RecvBitsCount() int64 {
	rate := int64(float64(t.totalRecvBits) / time.Now().Sub(t.startRecvTime).Seconds())
	t.totalRecvBits = 0
	t.startRecvTime = time.Now()
	return rate
}

/*
*****************************
/*     TCP communication      *
/*****************************
*/
type tcp struct {
	*transport
}

func (t *tcp) Listen() {
	log.Debug("start listening ", t.url.Port())
	listener, err := net.Listen("tcp", ":"+t.url.Port())
	if err != nil {
		log.Fatal("TCP Listener error: ", err)
	}
	t.startRecvTime = time.Now()

	go func(listener net.Listener) {
		defer listener.Close()
		for {
			conn, err := listener.Accept()

			if err != nil {
				log.Error("TCP Accept error: ", err)
				continue
			}

			// 使用远程地址作为协程标识符
			connName := conn.RemoteAddr().String()
			num := 0 //用来记录是第几个消息

			log.Debugf("Listen() --- 与 %v 建立连接成功", connName)
			go func(conn net.Conn, connName string) {
				// 创建解码器
				decoder := gob.NewDecoder(conn)
				defer conn.Close()

				for {
					select {
					case <-t.close:
						return
					default:
						log.Debugf("Listen() --- 当前协程正在监听来自 %v的 数据 No.[%v]", connName, num)
						var m interface{}
						err := decoder.Decode(&m)
						if err != nil {
							if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
								log.Warningf("Read timeout from %s for %T No.[%v]", conn.RemoteAddr().String(), m, num)
								time.Sleep(1 * time.Second)
								continue
							}
							log.Errorf("Decode error: [%v] from %v for type %T No.[%v]", err, connName, m, num)
							return
						}

						log.Debugf("Listen() --- 当前协程接受了%v的 数据 %T，准备计算接受大小 No.[%v]", connName, m, num)
						// 增加接收的数据量
						// var buf bytes.Buffer
						// enc := gob.NewEncoder(&buf)
						// enc.Encode(&m)

						// log.Debugf("Listen() --- 当前协程接受了%v的数据量为[%v]Byte No.[%v]", connName, buf.Len(), num)

						t.recv <- m
						//t.totalRecvBits += int64(buf.Len()) * 8
						num++
					}
				}
			}(conn, connName)
		}
	}(listener)
}

/*
*****************************
/*     UDP communication      *
/*****************************
*/
type udp struct {
	*transport
}

func (u *udp) Dial() error {
	addr, err := net.ResolveUDPAddr("udp", u.url.Host)
	if err != nil {
		log.Fatal("UDP resolve address error: ", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return err
	}

	u.startSendingTime = time.Now()

	go func(conn *net.UDPConn) {
		// packet := make([]byte, 1500)
		// w := bytes.NewBuffer(packet)
		w := new(bytes.Buffer)
		for m := range u.send {
			gob.NewEncoder(w).Encode(&m)
			_, err := conn.Write(w.Bytes())
			if err != nil {
				log.Error(err)
			}
			u.totalSentBits += int64(w.Len()) * 8
			w.Reset()
		}
	}(conn)

	return nil
}

func (u *udp) Listen() {
	addr, err := net.ResolveUDPAddr("udp", ":"+u.url.Port())
	if err != nil {
		log.Fatal("UDP resolve address error: ", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal("UDP Listener error: ", err)
	}
	u.startRecvTime = time.Now()
	go func(conn *net.UDPConn) {
		packet := make([]byte, 1500)
		defer conn.Close()
		for {
			select {
			case <-u.close:
				return
			default:
				_, err := conn.Read(packet)
				if err != nil {
					log.Error(err)
					continue
				}
				r := bytes.NewReader(packet)
				u.totalRecvBits += int64(r.Len()) * 8
				var m interface{}
				gob.NewDecoder(r).Decode(&m)
				u.recv <- m
			}
		}
	}(conn)
}

/*******************************
/* Intra-process communication *
/*******************************/

var chans = make(map[string]chan interface{})
var chansLock sync.RWMutex

type channel struct {
	*transport
}

func (c *channel) Scheme() string {
	return "chan"
}

func (c *channel) Dial() error {
	chansLock.RLock()
	defer chansLock.RUnlock()
	conn, ok := chans[c.url.Host]
	if !ok {
		return errors.New("server not ready")
	}
	c.startSendingTime = time.Now()
	go func(conn chan<- interface{}) {
		for m := range c.send {
			conn <- m
		}
	}(conn)
	return nil
}

func (c *channel) Listen() {
	chansLock.Lock()
	defer chansLock.Unlock()
	chans[c.url.Host] = make(chan interface{}, 1024)
	c.startRecvTime = time.Now()
	go func(conn <-chan interface{}) {
		for {
			select {
			case <-c.close:
				return
			case m := <-conn:
				c.recv <- m
			}
		}
	}(chans[c.url.Host])
}
