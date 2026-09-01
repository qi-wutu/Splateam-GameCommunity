// Go WebSocket 负载生成器 —— 用于探测服务端真正的单机连接上限。
//
// 为什么不用 k6 测连接数？k6 在本机跑到 ~700 并发就撞到了 k6 自身的 WS 调度/VU 上限
// （VU 稳定在 700/5000, ws_sessions 百万级 churn, 连接建立 avg 179ms），
// 而 Go 客户端用 goroutine + IOCP 能轻易开出上万条连接，测的才是服务端的真实容量。
//
// 用法（在 ws_test/ 下）：
//   node gen-tokens.mjs <N> "" load      # 先生成 N 个空 secret 的 token
//   go build -o ws_loadgen.exe ws_loadgen.go
//   ./ws_loadgen.exe -n 5000 -hold 30s
//
// 参数：
//   -n     尝试建立的连接数
//   -hold  每条连接保持时长
//   -tokens token 文件
//   -url   ws 端点
//   -dial  单条连接握手超时
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func readTokens(path string, need int) []string {
	f, err := os.Open(path)
	if err != nil {
		fmt.Println("❌ 打开 token 文件失败:", err)
		os.Exit(1)
	}
	defer f.Close()
	var tokens []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if t := sc.Text(); t != "" {
			tokens = append(tokens, t)
		}
	}
	if len(tokens) < need {
		fmt.Printf("⚠️  token 不足：需要 %d, 只有 %d。请用更大的数量重新生成。\n", need, len(tokens))
		os.Exit(1)
	}
	return tokens
}

func main() {
	n := flag.Int("n", 1000, "尝试建立的并发连接数")
	hold := flag.Duration("hold", 30*time.Second, "每条连接保持时长")
	tokensFile := flag.String("tokens", "tokens.txt", "token 文件（每行一个）")
	wsURLStr := flag.String("url", "ws://localhost:8080/api/ws", "websocket 端点")
	dialTimeout := flag.Duration("dial", 10*time.Second, "单条握手超时")
	rate := flag.Float64("rate", 0, "连接建立速率（个/秒），0=不限速全量突发")
	flag.Parse()

	tokens := readTokens(*tokensFile, *n)
	_ = mustParse(*wsURLStr) // 仅校验 url 格式

	dialer := &websocket.Dialer{HandshakeTimeout: *dialTimeout}

	var opened, failed, alive, peak int64
	var wg sync.WaitGroup

	// 采样器：每 2s 打印 alive，并跟踪峰值
	stop := make(chan struct{})
	samplerDone := make(chan struct{})
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				close(samplerDone)
				return
			case <-t.C:
				a := atomic.LoadInt64(&alive)
				updatePeak(&peak, a)
				fmt.Printf("alive=%d  已建连=%d  失败=%d  峰值=%d\n",
					a, atomic.LoadInt64(&opened), atomic.LoadInt64(&failed), atomic.LoadInt64(&peak))
			}
		}
	}()

	start := time.Now()
	var seq int64
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(tok string) {
			defer wg.Done()
			// 限速：按 -rate 均匀错开 dial 开始时间，模拟真实用户逐渐接入，避免突发打满 SYN 队列
			if *rate > 0 {
				idx := atomic.AddInt64(&seq, 1)
				target := start.Add(time.Duration(float64(idx-1) * float64(time.Second) / *rate))
				for time.Now().Before(target) {
					time.Sleep(2 * time.Millisecond)
				}
			}
			u := *wsURLStr + "?token=" + url.QueryEscape(tok)
			conn, _, err := dialer.Dial(u, nil)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				return
			}
			defer conn.Close()
			atomic.AddInt64(&opened, 1)
			atomic.AddInt64(&alive, 1)
			defer atomic.AddInt64(&alive, -1)

			conn.SetPongHandler(func(string) error { return nil })

			// 后台读，及时感知服务端关闭；同时保持心跳 alive
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			}()

			select {
			case <-time.After(*hold):
			case <-done:
			}
		}(tokens[i])
	}

	wg.Wait()
	close(stop)
	<-samplerDone
	updatePeak(&peak, atomic.LoadInt64(&alive))

	fmt.Printf("\n===== 连接压测结果 =====\n")
	fmt.Printf("尝试连接数: %d\n", *n)
	fmt.Printf("握手成功:   %d\n", atomic.LoadInt64(&opened))
	fmt.Printf("握手失败:   %d\n", atomic.LoadInt64(&failed))
	fmt.Printf("峰值同时在线: %d\n", atomic.LoadInt64(&peak))
	fmt.Printf("运行时长:   %v\n", time.Since(start).Round(time.Second))
}

func updatePeak(peak *int64, a int64) {
	for {
		p := atomic.LoadInt64(peak)
		if a <= p || atomic.CompareAndSwapInt64(peak, p, a) {
			return
		}
	}
}

func mustParse(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		fmt.Println("❌ url 解析失败:", err)
		os.Exit(1)
	}
	return u
}
