package main

import (
 "bufio"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net"
 "net/http"
 "net/url"
 "os"
 "os/exec"
 "path/filepath"
 "strconv"
 "strings"
 "time"
)

const (
 iptvRemotePlaylist = "https://iptv-org.github.io/iptv/countries/vn.m3u"
 iptvRemoteBackup = "https://raw.githubusercontent.com/iptv-org/iptv/master/streams/vn.m3u"
 iptvMaxPlaylistBytes = 2 << 20
 iptvMaxChannels = 180
 iptvMaxSources = 8
)
type iptvChannel struct {
 ID string
 Name string
 URLs []string
}
type iptvLoadResult struct {
 Channels []iptvChannel
 Err error
}
type iptvEvent struct {
 Session int64
 Message string
 Done bool
}
func iptvAcceptURL(raw string) bool {
 if len(raw)<10||len(raw)>2048||strings.ContainsAny(raw,"\r\n\t "){return false}
 u,err:=url.Parse(raw)
 if err!=nil || (u.Scheme!="https"&&u.Scheme!="http") || u.Hostname()=="" || u.User!=nil {return false}
 return true
}
func iptvAttribute(line,key string) string {
 search:=key+"=\""
 pos:=strings.Index(strings.ToLower(line),search)
 if pos<0{return ""}
 start:=pos+len(search)
 end:=strings.Index(line[start:],"\"")
 if end<0{return ""}
 return strings.TrimSpace(line[start:start+end])
}
func iptvNormalizeName(name string)string{
 name=strings.TrimSpace(name)
 for _,tag:=range []string{" [Geo-blocked]"," [Not 24/7]"," (1080p)"," (720p)"," (480p)"," (540p)"," (360p)"," (406p)"," (576p)"}{
  name=strings.ReplaceAll(name,tag,"")
 }
 return strings.TrimSpace(name)
}
func iptvChannelID(id,name string)string {
 id=strings.TrimSpace(id)
 // SD, HD and FHD editions of the same tvg-id must share fallbacks.
 if i:=strings.IndexByte(id,'@');i>=0{id=id[:i]}
 if id!="" {return strings.ToLower(id)}
 return "name:"+strings.ToLower(strings.Join(strings.Fields(iptvNormalizeName(name))," "))
}
func iptvParseM3U(r io.Reader)[]iptvChannel{
 result:=make([]iptvChannel,0,64)
 index:=make(map[string]int)
 scanner:=bufio.NewScanner(io.LimitReader(r,iptvMaxPlaylistBytes+1))
 scanner.Buffer(make([]byte,4096),64*1024)
 id,name:="",""
 for scanner.Scan(){
  line:=strings.TrimSpace(scanner.Text())
  if strings.HasPrefix(line,"#EXTINF:"){
   id=iptvAttribute(line,"tvg-id")
   name=iptvAttribute(line,"tvg-name")
   if comma:=strings.Index(line,",");comma>=0 && strings.TrimSpace(line[comma+1:])!="" {
    name=strings.TrimSpace(line[comma+1:])
   }
   name=iptvNormalizeName(name)
   continue
  }
  if strings.HasPrefix(line,"#")||line==""{continue}
  if !iptvAcceptURL(line)||name=="" {id="";name="";continue}
  key:=iptvChannelID(id,name)
  if key=="name:"{id="";name="";continue}
  idx,exists:=index[key]
  if !exists {
   if len(result)>=iptvMaxChannels {break}
   idx=len(result);index[key]=idx
   result=append(result,iptvChannel{ID:key,Name:name})
  }
  if len(result[idx].URLs)<iptvMaxSources{
   dup:=false
   for _,current:=range result[idx].URLs{if current==line{dup=true;break}}
   if !dup{result[idx].URLs=append(result[idx].URLs,line)}
  }
  id="";name=""
 }
 return result
}
func iptvMerge(lists ...[]iptvChannel)[]iptvChannel{
 result:=make([]iptvChannel,0,64)
 index:=map[string]int{}
 for _,xs:=range lists {
  for _,ch:=range xs {
   if ch.ID==""||len(ch.URLs)==0{continue}
   n,found:=index[ch.ID]
   if !found{
    if len(result)>=iptvMaxChannels{continue}
    n=len(result);index[ch.ID]=n
    result=append(result,iptvChannel{ID:ch.ID,Name:ch.Name})
   }
   for _,raw:=range ch.URLs{
    if !iptvAcceptURL(raw)||len(result[n].URLs)>=iptvMaxSources{continue}
    dup:=false
    for _,old:=range result[n].URLs{if old==raw{dup=true;break}}
    if !dup{result[n].URLs=append(result[n].URLs,raw)}
   }
  }
 }
 return result
}
func iptvFetchSource(ctx context.Context,client *http.Client,address string)([]iptvChannel,error){
 if !iptvAcceptURL(address){return nil,errors.New("URL playlist không hợp lệ")}
 req,err:=http.NewRequestWithContext(ctx,http.MethodGet,address,nil)
 if err!=nil{return nil,err}
 req.Header.Set("User-Agent","Binance-TrimUI-IPTV/1.0")
 resp,err:=client.Do(req)
 if err!=nil{return nil,err}
 defer resp.Body.Close()
 if resp.StatusCode!=http.StatusOK {return nil,fmt.Errorf("HTTP %d",resp.StatusCode)}
 b,err:=io.ReadAll(io.LimitReader(resp.Body,iptvMaxPlaylistBytes+1))
 if err!=nil{return nil,err}
 if len(b)>iptvMaxPlaylistBytes{return nil,errors.New("playlist quá lớn")}
 if !strings.Contains(string(b),"#EXTM3U"){return nil,errors.New("không phải playlist M3U")}
 channels:=iptvParseM3U(strings.NewReader(string(b)))
 if len(channels)==0{return nil,errors.New("playlist không có luồng được hỗ trợ")}
 return channels,nil
}
func iptvLoad(ctx context.Context,client *http.Client,local string)iptvLoadResult{
 var lists [][]iptvChannel
 errorsSeen:=[]string{}
 if b,err:=os.ReadFile(local);err==nil{
  if len(b)<=iptvMaxPlaylistBytes{
   lists=append(lists,iptvParseM3U(strings.NewReader(string(b))))
  }else{errorsSeen=append(errorsSeen,"IPTV cục bộ quá lớn")}
 }
 for _,address:=range []string{iptvRemotePlaylist,iptvRemoteBackup}{
  xs,err:=iptvFetchSource(ctx,client,address)
  if err!=nil{errorsSeen=append(errorsSeen,err.Error());continue}
  lists=append(lists,xs)
 }
 merged:=iptvMerge(lists...)
 if len(merged)==0{
  return iptvLoadResult{Err:fmt.Errorf("không tải được danh sách IPTV (%s)",strings.Join(errorsSeen,"; "))}
 }
 return iptvLoadResult{Channels:merged}
}
func iptvFindByID(channels []iptvChannel,id string)(iptvChannel,bool){
 for _,ch:=range channels{if ch.ID==id{return ch,true}}
 return iptvChannel{},false
}
func iptvAvailablePlayer()(string,error){
 if p,err:=exec.LookPath("mpv");err==nil{return p,nil}
 // User may provide an ARM64-compatible player in the app folder.
 candidate:=filepath.Join(appDir(),"mpv")
 if info,err:=os.Stat(candidate);err==nil && !info.IsDir() && info.Mode()&0111!=0 {
  return candidate,nil
 }
 return "",errors.New("không tìm thấy MPV trên Stock OS. Cần trình phát MPV ARM64 tương thích để xem TV")
}
type iptvMPVReply struct {
 Error string `json:"error"`
 Data json.RawMessage `json:"data"`
 RequestID int `json:"request_id"`
}
func iptvMPVProperty(sock,property string)(json.RawMessage,error){
 conn,err:=net.DialTimeout("unix",sock,350*time.Millisecond)
 if err!=nil{return nil,err}
 defer conn.Close()
 _=conn.SetDeadline(time.Now().Add(900*time.Millisecond))
 query,_:=json.Marshal(map[string]any{"command":[]any{"get_property",property},"request_id":1327})
 if _,err=conn.Write(append(query,byte(10)));err!=nil{return nil,err}
 reader:=bufio.NewReader(io.LimitReader(conn,8192))
 for n:=0;n<8;n++{
  line,err:=reader.ReadBytes(byte(10))
  if err!=nil{return nil,err}
  var reply iptvMPVReply
  if json.Unmarshal(line,&reply)==nil&&reply.RequestID==1327{
   if reply.Error!="success"{return nil,errors.New(reply.Error)}
   return reply.Data,nil
  }
 }
 return nil,errors.New("MPV không phản hồi IPC")
}
func iptvPlaySource(ctx context.Context,skip <-chan struct{},player,address string)(time.Duration,error){
 if !iptvAcceptURL(address){return 0,errors.New("luồng IPTV không hợp lệ")}
 sock:=filepath.Join(os.TempDir(),fmt.Sprintf("binance-iptv-%d-%d.sock",os.Getpid(),time.Now().UnixNano()))
 defer os.Remove(sock)
 args:=[]string{
  "--no-config","--really-quiet","--fullscreen","--idle=no","--keep-open=no",
  "--input-terminal=no","--input-default-bindings=no",
  "--network-timeout=10","--cache=yes","--demuxer-readahead-secs=3",
  "--input-ipc-server="+sock,"--",address,
 }
 cmd:=exec.CommandContext(ctx,player,args...)
 // Do not inherit stdin or /dev/input; the BINANCE app handles D-pad/B itself.
 cmd.Stdin=nil
 if err:=cmd.Start();err!=nil{return 0,err}
 started:=time.Now()
 done:=make(chan error,1)
 go func(){done<-cmd.Wait()}()
 tick:=time.NewTicker(4*time.Second)
 defer tick.Stop()
 var lastPos float64
 var lastAdvance time.Time
 var cacheSince time.Time
 everAdvanced:=false
 for {
  select{
  case err:=<-done:
   if ctx.Err()!=nil{return time.Since(started),ctx.Err()}
   if err==nil {return time.Since(started),errors.New("luồng đã kết thúc")}
   return time.Since(started),fmt.Errorf("MPV: %w",err)
  case <-skip:
   if cmd.Process!=nil{_ = cmd.Process.Kill()}
   select{case <-done:case <-time.After(2*time.Second):}
   return time.Since(started),errors.New("đã chuyển nguồn thủ công")
  case <-ctx.Done():
   if cmd.Process!=nil{_ = cmd.Process.Kill()}
   select{case <-done:case <-time.After(2*time.Second):}
   return time.Since(started),ctx.Err()
  case <-tick.C:
   // Playback clocks and cache state allow a bounded stall timeout.
   // If a firmware MPV has no IPC/time-pos, do not terminate healthy video.
   raw,err:=iptvMPVProperty(sock,"time-pos")
   if err==nil{
    var pos float64
    if json.Unmarshal(raw,&pos)==nil{
     if !everAdvanced||pos>lastPos+0.25 {
      lastPos=pos;lastAdvance=time.Now();everAdvanced=true
     }else if everAdvanced&&time.Since(lastAdvance)>25*time.Second{
      if cmd.Process!=nil{_ = cmd.Process.Kill()}
      <-done
      return time.Since(started),errors.New("luồng video bị đứng quá 25 giây")
     }
    }
   }
   cache,err:=iptvMPVProperty(sock,"paused-for-cache")
   if err==nil {
    var waiting bool
    if json.Unmarshal(cache,&waiting)==nil&&waiting{
     if cacheSince.IsZero(){cacheSince=time.Now()}
     if time.Since(cacheSince)>22*time.Second{
      if cmd.Process!=nil{_ = cmd.Process.Kill()}
      <-done
      return time.Since(started),errors.New("luồng bị mất dữ liệu quá 22 giây")
     }
    }else{cacheSince=time.Time{}}
   }
  }
 }
}
func iptvPlayback(ctx context.Context,skip <-chan struct{},session int64,ch iptvChannel,client *http.Client,local string,events chan<- iptvEvent) {
 lastStatus:="ĐÃ DỪNG IPTV"
 emit:=func(msg string,done bool){
  if msg!=""{lastStatus=msg}
  event:=iptvEvent{Session:session,Message:msg,Done:done}
  select{case events<-event:case <-ctx.Done():}
 }
 defer func(){
  if ctx.Err()!=nil{lastStatus="ĐÃ DỪNG IPTV"}
  select {case events<-iptvEvent{Session:session,Message:lastStatus,Done:true}:default:}
 }()
 player,err:=iptvAvailablePlayer()
 if err!=nil{emit(err.Error(),false);return}
 tried:=make(map[string]bool)
 sources:=append([]string(nil),ch.URLs...)
 refreshed:=false
 attempt:=0
 for ctx.Err()==nil {
  var next string
  for _,address:=range sources{if !tried[address]&&iptvAcceptURL(address){next=address;break}}
  if next==""{
   if refreshed {emit("KÊNH TẠM KHÔNG CÓ NGUỒN PHÁT HOẠT ĐỘNG",false);return}
   refreshed=true
   emit("ĐANG TẢI LẠI DANH SÁCH ĐỂ TÌM LINK MỚI...",false)
   result:=iptvLoad(ctx,client,local)
   if result.Err!=nil{emit("KHÔNG TÌM ĐƯỢC LINK DỰ PHÒNG: "+result.Err.Error(),false);return}
   found,ok:=iptvFindByID(result.Channels,ch.ID)
   if !ok {emit("KHÔNG CÒN KÊNH TRONG DANH SÁCH CẬP NHẬT",false);return}
   sources=found.URLs
   continue
  }
  tried[next]=true
  attempt++
  emit(fmt.Sprintf("ĐANG PHÁT %s - NGUỒN %d (B ĐỂ DỪNG)",ch.Name,attempt),false)
  _,err:=iptvPlaySource(ctx,skip,player,next)
  if ctx.Err()!=nil{return}
  emit(fmt.Sprintf("NGUỒN %d LỖI: %s - TỰ CHUYỂN NGUỒN",attempt,cutNews(err.Error(),44)),false)
 }
}
