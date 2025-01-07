package transport

import (
	"bytes"
	"crypto/tls"
	"encoding/gob"
	"errors"
	"time"

	"github.com/gitferry/bamboo/log"
	"github.com/lucas-clemente/quic-go"
)

type quicTransport struct {
	*transport
	listener quic.Listener
}

func (q *quicTransport) Dial() error {
	session, err := quic.DialAddr(q.url.Host, generateTLSConfig(), generateQUICConfig())
	if err != nil {
		log.Errorf("QUIC Dial error: %v", err)
		return err
	}
	q.startSendingTime = time.Now()

	go func(session quic.Session) {
		defer session.CloseWithError(0, errors.New("closing session"))
		stream, err := session.OpenStream()
		if err != nil {
			log.Errorf("QUIC OpenStream error: %v", err)
			return
		}

		encoder := gob.NewEncoder(stream)
		num := 0
		for m := range q.send {
			if err := encoder.Encode(&m); err != nil {
				log.Errorf("QUIC Send error: %v", err)
				return
			}

			<-q.concurrentLimit
			log.Debugf("Dial() --- 发送到[%v]成功, 限流器大小[%v] 发送数据为%T,准备写入本地缓冲区来获取发送的数据量 No.[%v]", session.RemoteAddr(), len(q.concurrentLimit), m, num)
			num++
			var buf bytes.Buffer
			enc := gob.NewEncoder(&buf)
			enc.Encode(&m)
			q.totalSentBits += int64(buf.Len()) * 8
		}
	}(session)

	return nil
}

func (q *quicTransport) Listen() {
	listener, err := quic.ListenAddr(":"+q.url.Port(), generateTLSConfig(), nil)
	if err != nil {
		log.Fatalf("QUIC Listener error: %v", err)
	}
	q.listener = listener
	q.startRecvTime = time.Now()

	go func(listener quic.Listener) {
		defer listener.Close()
		for {
			session, err := listener.Accept()
			if err != nil {
				log.Errorf("QUIC Accept error: %v", err)
				return
			}

			go func(session quic.Session) {
				defer session.CloseWithError(0, errors.New("closing session"))
				stream, err := session.AcceptStream()
				if err != nil {
					log.Errorf("QUIC AcceptStream error: %v", err)
					return
				}

				decoder := gob.NewDecoder(stream)
				for {
					var m interface{}
					if err := decoder.Decode(&m); err != nil {
						log.Errorf("QUIC Receive error: %v", err)
						return
					}
					q.recv <- m

					var buf bytes.Buffer
					enc := gob.NewEncoder(&buf)
					enc.Encode(&m)
					q.totalRecvBits += int64(buf.Len()) * 8
				}
			}(session)
		}
	}(listener)
}

func generateTLSConfig() *tls.Config {
	// 加载证书和私钥
	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		log.Fatalf("failed to load certificates: %v", err)
	}

	return &tls.Config{
		InsecureSkipVerify: true,                    // 忽略证书验证
		Certificates:       []tls.Certificate{cert}, // 设置证书
		NextProtos:         []string{"bamboo-quic"}, // 设置协议
	}
}

func generateQUICConfig() *quic.Config {
	//
	return &quic.Config{
		HandshakeTimeout: 5 * time.Second,  // 设置握手超时时间
		IdleTimeout:      30 * time.Minute, // 设置空闲超时时间
	}
}
