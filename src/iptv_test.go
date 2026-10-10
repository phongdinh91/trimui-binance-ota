package main

import (
 "context"
 "io"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func TestIPTVParseSameChannelHasBackup(t *testing.T){
 input:=`#EXTM3U
#EXTINF:-1 tvg-id="VTV1.vn@HD",VTV1 HD (1080p)
https://a.example.org/a.m3u8
#EXTINF:-1 tvg-id="VTV1.vn@SD",VTV1 SD (480p)
https://b.example.org/b.m3u8
#EXTINF:-1 tvg-id="VTV1.vn@HD",VTV1 duplicate
https://a.example.org/a.m3u8
#EXTINF:-1 tvg-id="VTV2.vn@HD",VTV2
file:///tmp/video
#EXTINF:-1 tvg-id="VTV2.vn@HD",VTV2
http://c.example.org/live.m3u8
`
 channels:=iptvParseM3U(strings.NewReader(input))
 if len(channels)!=2{t.Fatalf("expected 2 channels, got %+v",channels)}
 if channels[0].ID!="vtv1.vn"||len(channels[0].URLs)!=2{
  t.Fatalf("VTV1 backup not grouped: %+v",channels[0])
 }
 if channels[1].ID!="vtv2.vn"||len(channels[1].URLs)!=1{
  t.Fatalf("bad protocol should be skipped: %+v",channels[1])
 }
}

func TestIPTVNeverMixOtherChannels(t *testing.T){
 lists:=[][]iptvChannel{
  {{ID:"vtv1.vn",Name:"VTV1",URLs:[]string{"https://a.tv/1"}}},
  {{ID:"vtv2.vn",Name:"VTV2",URLs:[]string{"https://b.tv/2"}},{ID:"vtv1.vn",Name:"VTV1",URLs:[]string{"https://c.tv/1"}}},
 }
 merged:=iptvMerge(lists...)
 if len(merged)!=2{t.Fatalf("got %d channels",len(merged))}
 ch,ok:=iptvFindByID(merged,"vtv1.vn")
 if !ok||len(ch.URLs)!=2||ch.URLs[1]!="https://c.tv/1"{t.Fatal("same channel fallback missing")}
 ch2,ok:=iptvFindByID(merged,"vtv2.vn")
 if !ok||len(ch2.URLs)!=1{t.Fatal("must never failover to a different channel")}
}

func TestIPTVURLRejectsLocalAndUnsupportedProtocols(t *testing.T){
 for _,raw:=range []string{"file:///etc/passwd","javascript:alert(1)","https://user:pass@tv.test/live","https://host.test/x"+string(rune(10))+"--bad",""}{
  if iptvAcceptURL(raw){t.Fatalf("accepted invalid URL %q",raw)}
 }
 for _,raw:=range []string{"https://stream.example.org/live.m3u8","http://stream.example.org/live.m3u8"}{
  if !iptvAcceptURL(raw){t.Fatalf("rejected valid URL %q",raw)}
 }
}

func TestIPTVFetchBoundedPlaylist(t *testing.T) {
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  io.WriteString(w,`#EXTM3U
#EXTINF:-1 tvg-id="TV1.vn",TV1
https://example.org/stream.m3u8
`)
 }))
 defer server.Close()
 xs,err:=iptvFetchSource(context.Background(),server.Client(),server.URL)
 if err!=nil||len(xs)!=1{t.Fatalf("fetch: %v %+v",err,xs)}
}

func TestIPTVFallbackAfterBrokenMPVSource(t *testing.T){
 dir:=t.TempDir()
 script:=filepath.Join(dir,"mpv")
 // Shell MPV stub: first URL fails, second stays alive until cancelled.
 // This tests the failover engine without device sysfs or a video codec.
 body:=`#!/bin/sh
case "$*" in *broken.m3u8*) exit 7 ;; *) /bin/sleep 10 ;; esac
`
 if err:=os.WriteFile(script,[]byte(body),0755);err!=nil{t.Fatal(err)}
 t.Setenv("PATH",dir)
 ctx,cancel:=context.WithTimeout(context.Background(),4*time.Second)
 defer cancel()
 ch:=iptvChannel{ID:"test.vn",Name:"Demo",URLs:[]string{"https://example.org/broken.m3u8","https://example.org/backup.m3u8"}}
 events:=make(chan iptvEvent,32)
 done:=make(chan struct{})
 go func(){iptvPlayback(ctx,make(chan struct{}),34,ch,&http.Client{Timeout:time.Second},"",events);close(done)}()
 first,second:=false,false
 for !second {
  select{
  case ev:=<-events:
   if strings.Contains(ev.Message,"NGUỒN 1"){first=true}
   if strings.Contains(ev.Message,"NGUỒN 2"){second=true}
   if ev.Done && !second{t.Fatalf("fallback never started: %+v",ev)}
  case <-ctx.Done():t.Fatal("timed out waiting for fallback")
  }
 }
 if !first{t.Fatal("no primary source attempt")}
 cancel()
 select{case <-done:case <-time.After(2*time.Second):t.Fatal("MPV did not exit on cancellation")}
}
