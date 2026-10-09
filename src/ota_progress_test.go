package main

import (
 "archive/zip"
 "bytes"
 "crypto/sha256"
 "encoding/hex"
 "io"
 "net/http"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

type fakeOTAHTTP func(*http.Request)(*http.Response,error)
func(f fakeOTAHTTP)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}

func TestOTAPercentClampsAndUnknown(t *testing.T){
 samples:=[]struct{p otaProgressEvent;want int;known bool}{
  {otaProgressEvent{Done:0,Total:-1},0,false},
  {otaProgressEvent{Done:0,Total:100},0,true},
  {otaProgressEvent{Done:31,Total:100},31,true},
  {otaProgressEvent{Done:100,Total:100},100,true},
  {otaProgressEvent{Done:120,Total:100},100,true},
  {otaProgressEvent{Done:-10,Total:100},0,true},
 }
 for _,q:=range samples {got,known:=otaPercent(q.p);if got!=q.want||known!=q.known {t.Fatalf("%+v => %d,%t want %d,%t",q.p,got,known,q.want,q.known)}}
}

func TestDownloadOTAProgressAndHash(t *testing.T){
 data:=bytes.Repeat([]byte("binance-ota-test"),33000)
 sum:=sha256.Sum256(data)
 m:=otaManifest{URL:"https://example.test/release.zip",SHA256:hex.EncodeToString(sum[:])}
 reqCount:=0
 client:=&http.Client{Transport:fakeOTAHTTP(func(r *http.Request)(*http.Response,error){
  reqCount++
  if r.URL.String()!=m.URL {t.Fatalf("wrong URL: %s",r.URL)}
  return &http.Response{StatusCode:200,Body:io.NopCloser(bytes.NewReader(data)),ContentLength:int64(len(data)),Header:make(http.Header)},nil
 })}
 dst:=filepath.Join(t.TempDir(),"release.zip")
 var events []otaProgressEvent
 err:=downloadOTAWithProgress(client,m,dst,func(p otaProgressEvent){events=append(events,p)})
 if err!=nil {t.Fatal(err)}
 disk,err:=os.ReadFile(dst);if err!=nil {t.Fatal(err)}
 if !bytes.Equal(disk,data) {t.Fatal("download mismatch")}
 if reqCount!=1||len(events)<4 {t.Fatalf("req %d events %d",reqCount,len(events))}
 if first:=events[0];first.Stage!="download"||first.Done!=0 {t.Fatalf("first event: %+v",first)}
 sawVerify:=false
 var highest int64
 for _,p:=range events {
  if p.Stage=="verify" {sawVerify=true}
  if p.Stage=="download" {if p.Done<highest {t.Fatal("progress regressed")};highest=p.Done}
 }
 if !sawVerify || highest!=int64(len(data)) {t.Fatalf("events missing finish %d %+v",highest,events[len(events)-1])}
}

func TestOTARejectsBadDigest(t *testing.T){
 payload:=[]byte("wrong checksum")
 m:=otaManifest{URL:"https://example.test/ota.zip",SHA256:strings.Repeat("0",64)}
 client:=&http.Client{Transport:fakeOTAHTTP(func(r *http.Request)(*http.Response,error){
  return &http.Response{StatusCode:200,Body:io.NopCloser(bytes.NewReader(payload)),ContentLength:int64(len(payload)),Header:make(http.Header)},nil
 })}
 dst:=filepath.Join(t.TempDir(),"bad.zip")
 if err:=downloadOTAWithProgress(client,m,dst,nil);err==nil {t.Fatal("bad digest accepted")}
 if _,err:=os.Stat(dst);!os.IsNotExist(err) {t.Fatalf("bad digest file retained: %v",err)}
}

func TestOTAExtractionReportsFileProgress(t *testing.T) {
 temp:=t.TempDir()
 archive:=filepath.Join(temp,"ota.zip")
 f,err:=os.Create(archive);if err!=nil {t.Fatal(err)}
 zw:=zip.NewWriter(f)
 entries:=map[string]string{
  "Apps/BinanceGia.pak/binance-gia":"ARM64 mock binary",
  "Apps/BinanceGia.pak/launch.sh":"#!/bin/sh\n",
  "Apps/BinanceGia.pak/config.json": "{\"label\":\"Binance\"}",
  "Apps/BinanceGia.pak/assets/example.txt":"asset",
 }
 for name,data:=range entries {
  w,e:=zw.Create(name);if e!=nil {t.Fatal(e)}
  if _,e=w.Write([]byte(data));e!=nil {t.Fatal(e)}
 }
 if err:=zw.Close();err!=nil {t.Fatal(err)}
 if err:=f.Close();err!=nil {t.Fatal(err)}
 var last otaProgressEvent
 dst:=filepath.Join(temp,"stage")
 if err:=extractOTAWithProgress(archive,dst,func(p otaProgressEvent){last=p});err!=nil {t.Fatal(err)}
 if last.Stage!="extract"||last.Done!=int64(len(entries))||last.Total!=int64(len(entries)) {t.Fatalf("extraction progress: %+v",last)}
 if _,err:=os.Stat(filepath.Join(dst,"launch.sh"));err!=nil {t.Fatal(err)}
}
