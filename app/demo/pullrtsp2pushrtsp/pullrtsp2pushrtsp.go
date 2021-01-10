// Copyright 2020, Chef.  All rights reserved.
// https://github.com/q191201771/lal
//
// Use of this source code is governed by a MIT-style license
// that can be found in the License file.
//
// Author: Chef (191201771@qq.com)

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/q191201771/lal/pkg/base"
	"github.com/q191201771/lal/pkg/rtprtcp"
	"github.com/q191201771/lal/pkg/rtsp"
	"github.com/q191201771/naza/pkg/nazalog"
)

var rtpPacketChan = make(chan rtprtcp.RTPPacket, 1024)

type Observer struct {
}

func (o *Observer) OnRTPPacket(pkt rtprtcp.RTPPacket) {
	rtpPacketChan <- pkt
}

func (o *Observer) OnAVConfig(asc, vps, sps, pps []byte) {
	// noop
}

func (o *Observer) OnAVPacket(pkt base.AVPacket) {
	// noop
}

func main() {
	_ = nazalog.Init(func(option *nazalog.Option) {
		option.AssertBehavior = nazalog.AssertFatal
	})

	inURL, outURL, pullOverTCP, pushOverTCP := parseFlag()

	o := &Observer{}
	rtspPullSession := rtsp.NewPullSession(o, func(option *rtsp.PullSessionOption) {
		option.PullTimeoutMS = 5000
		option.OverTCP = pullOverTCP != 0
	})

	rtspPushSession := rtsp.NewPushSession(func(option *rtsp.PushSessionOption) {
		option.PushTimeoutMS = 5000
		option.OverTCP = pushOverTCP != 0
	})

	go func() {
		time.Sleep(3 * time.Second)
		for {
			rtspPullSession.UpdateStat(1)
			pullStat := rtspPullSession.GetStat()
			rtspPushSession.UpdateStat(1)
			pushStat := rtspPushSession.GetStat()
			nazalog.Debugf("stat. pull=%+v, push=%+v", pullStat, pushStat)
			time.Sleep(1 * time.Second)
		}
	}()

	err := rtspPullSession.Pull(inURL)
	nazalog.Assert(nil, err)
	defer rtspPullSession.Dispose()
	rawSDP, sdpLogicCtx := rtspPullSession.GetSDP()

	err = rtspPushSession.Push(outURL, rawSDP, sdpLogicCtx)
	nazalog.Assert(nil, err)
	defer rtspPushSession.Dispose()

	// 只是为了测试主动关闭session
	//go func() {
	//	time.Sleep(5 * time.Second)
	//	rtspPullSession.Dispose()
	//}()

	for {
		select {
		case err = <-rtspPullSession.Wait():
			nazalog.Infof("< pullSession.Wait(). err=%+v", err)
			time.Sleep(1 * time.Second) // 不让程序立即退出，只是为了测试session内部资源是否正常及时释放
			return
		case err = <-rtspPushSession.Wait():
			nazalog.Infof("< pushSession.Wait(). err=%+v", err)
			time.Sleep(1 * time.Second)
			return
		case pkt := <-rtpPacketChan:
			rtspPushSession.WriteRTPPacket(pkt)
		}
	}

}

func parseFlag() (inURL string, outURL string, pullOverTCP int, pushOverTCP int) {
	i := flag.String("i", "", "specify pull rtsp url")
	o := flag.String("o", "", "specify push rtmp url")
	t := flag.Int("t", 0, "specify pull interleaved mode(rtp/rtcp over tcp)")
	y := flag.Int("y", 0, "specify push interleaved mode(rtp/rtcp over tcp)")
	flag.Parse()
	if *i == "" || *o == "" {
		flag.Usage()
		_, _ = fmt.Fprintf(os.Stderr, `Example:
  ./bin/pullrtsp2pushrtsp -i rtsp://localhost:5544/live/test110 -o rtsp://localhost:5544/live/test220
  ./bin/pullrtsp2pushrtsp -i rtsp://localhost:5544/live/test110 -o rtsp://localhost:5544/live/test220 -t 1 -y 1
`)
		os.Exit(1)
	}
	return *i, *o, *t, *y
}