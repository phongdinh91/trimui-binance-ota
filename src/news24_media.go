package main

import (
 "bytes"
 "errors"
 "fmt"
 "html"
 "image"
 _ "image/gif"
 _ "image/jpeg"
 "io"
 "net/http"
 "net/url"
 "os/exec"
 "path"
 "regexp"
 "strings"
)

type newsBlock struct {
 Kind string // "text", "image", "video"
 Text string
 URL string
 Poster string
}

var (
 newsBodyStart=regexp.MustCompile("(?is)<(?:div|article|section)\\b[^>]*(?:text-conent|text-content|article-body|articleBody|article__content|article-content|detail-content|content-news-detail|articleBody)[^>]*>")
 newsNodeStart=regexp.MustCompile("(?is)<(p|h2|h3|img|video|source|iframe)\\b[^>]*>")
 newsAttrRe=regexp.MustCompile("(?is)([a-zA-Z_][a-zA-Z0-9_:-]*)\\s*=\\s*(?:\"([^\"]*)\"|'([^']*)'|([^\\s>]+))")
 newsImageMeta=regexp.MustCompile("(?is)<meta\\b[^>]*property\\s*=\\s*[\"']og:image[\"'][^>]*>")
)
func newsAttribute(raw string, names ...string)string {
 attrs:=map[string]string{}
 for _,m:=range newsAttrRe.FindAllStringSubmatch(raw,-1){
  v:=m[2];if v==""{v=m[3]};if v==""{v=m[4]}
  attrs[strings.ToLower(m[1])]=html.UnescapeString(strings.TrimSpace(v))
 }
 for _,name:=range names {
  v:=attrs[name]
  if strings.Contains(name,"srcset") {
   v=strings.Fields(strings.Split(v,",")[0])[0]
  }
  if v!=""&&!strings.HasPrefix(v,"data:"){return v}
 }
 return ""
}
func newsResolveMedia(base,raw string)string {
 raw=strings.TrimSpace(raw)
 if strings.HasPrefix(raw,"//"){raw="https:"+raw}
 link,err:=url.Parse(raw)
 if err!=nil{return ""}
 source,err:=url.Parse(base)
 if err!=nil{return ""}
 resolved:=source.ResolveReference(link)
 if resolved.Scheme!="https"||resolved.User!=nil{return ""}
 host:=strings.ToLower(resolved.Hostname())
 // Never request localhost or private hosts from remote article markup.
 if host!="24h.com.vn"&&!strings.HasSuffix(host,".24h.com.vn") {
  // Third-party video embeds are recorded for identification but never downloaded.
  return ""
 }
 resolved.Fragment=""
 return resolved.String()
}
func newsExternalVideo(raw string)string {
 u,err:=url.Parse(strings.TrimSpace(raw))
 if err!=nil||u.Scheme!="https"||u.User!=nil{return ""}
 host:=strings.ToLower(u.Hostname())
 if host=="www.youtube.com"||host=="youtube.com"||host=="www.youtube-nocookie.com"||host=="player.vimeo.com" {return u.String()}
 return ""
}
func newsBodyRegion(page string)string {
 // 24h commonly uses the misspelled "text-conent" wrapper.
 clean:=goldScriptsRe.ReplaceAllString(page," ")
 if m:=newsBodyStart.FindStringIndex(clean);len(m)==2{return clean[m[0]:]}
 if h:=newsTitleTag.FindStringIndex(clean);len(h)==2{return clean[h[1]:]}
 return clean
}
func newsMediaVideo(raw string) bool {
 u,err:=url.Parse(raw);if err!=nil{return false}
 ext:=strings.ToLower(path.Ext(u.Path))
 return ext==".mp4"||ext==".m3u8"||ext==".webm"
}

// Reads the article's body paragraphs in source order instead of only its meta description.
// The source site's article markup can change; no text/media is fabricated.
func parse24hRichArticle(page,base string) newsArticle {
 out:=parse24hArticleText(page)
 content:=newsBodyRegion(page)
 matched:=newsNodeStart.FindAllStringIndex(content,-1)
 seenMedia:=map[string]bool{}
 totalText:=0
 for i:=0;i<len(matched)&&len(out.Blocks)<90;i++ {
  start,end:=matched[i][0],matched[i][1]
  tag:=content[start:end]
  tagName:=strings.ToLower(strings.Fields(strings.TrimPrefix(tag,"<"))[0])
  tagName=strings.TrimRight(tagName,">")
  switch tagName {
  case "p","h2","h3":
   closeTag:="</"+tagName+">"
   close:=strings.Index(strings.ToLower(content[end:]),closeTag)
   if close<0||close>14000{continue}
   inner:=content[end:end+close]
   if strings.Contains(strings.ToLower(inner),"<img")||strings.Contains(strings.ToLower(inner),"<video"){
    continue // allow media inside the enclosing paragraph to be found separately
   }
   line:=tidyNewsText(inner)
   if len([]rune(line))<16||len([]rune(line))>3000{continue}
   low:=strings.ToLower(line)
   if strings.HasPrefix(low,"xem thêm:")||strings.HasPrefix(low,"quảng cáo") {continue}
   if totalText+len([]rune(line))>42000{break}
   out.Blocks=append(out.Blocks,newsBlock{Kind:"text",Text:line})
   totalText+=len([]rune(line))
  case "img":
   src:=newsAttribute(tag,"data-original","data-src","data-lazy-src","data-url","src")
   link:=newsResolveMedia(base,src)
   if link==""||seenMedia[link] {continue}
   seenMedia[link]=true
   out.Blocks=append(out.Blocks,newsBlock{Kind:"image",URL:link,Text:tidyNewsText(newsAttribute(tag,"alt","title"))})
  case "video","source","iframe":
   src:=newsAttribute(tag,"data-video-src","data-src","src")
   link:=newsResolveMedia(base,src)
   if link==""{link=newsExternalVideo(src)}
   if link==""||seenMedia[link]{continue}
   seenMedia[link]=true
   poster:=newsResolveMedia(base,newsAttribute(tag,"poster","data-poster","data-thumb"))
   out.Blocks=append(out.Blocks,newsBlock{Kind:"video",URL:link,Poster:poster,Text:"VIDEO TỪ 24H"})
  }
 }
 if len(out.Blocks)==0&&out.Excerpt!=""{
  out.Blocks=[]newsBlock{{Kind:"text",Text:out.Excerpt}}
 }
 if len(out.Blocks)>0{
  // Do not incorrectly pretend a meta summary is the entire article.
  out.Excerpt=out.Blocks[0].Text
 }
 return out
}
func newsImageForBlock(b newsBlock)string {
 if b.Kind=="image"{return b.URL}
 if b.Kind=="video"{return b.Poster}
 return ""
}
func fetchNewsPicture(client *http.Client, uri string)(*asset,error) {
 if newsResolveMedia("https://www.24h.com.vn/",uri)!=uri{return nil,errors.New("URL ảnh không hợp lệ")}
 req,err:=http.NewRequest(http.MethodGet,uri,nil);if err!=nil{return nil,err}
 req.Header.Set("User-Agent","Mozilla/5.0")
 req.Header.Set("Referer","https://www.24h.com.vn/")
 resp,err:=client.Do(req);if err!=nil{return nil,err}
 defer resp.Body.Close()
 if resp.StatusCode!=200{return nil,fmt.Errorf("ảnh HTTP %d",resp.StatusCode)}
 data,err:=io.ReadAll(io.LimitReader(resp.Body,4*1024*1024+1))
 if err!=nil{return nil,err}
 if len(data)>4*1024*1024{return nil,errors.New("ảnh quá lớn")}
 img,_,err:=image.Decode(bytes.NewReader(data))
 if err!=nil{return nil,err}
 bounds:=img.Bounds()
 if bounds.Dx()<8||bounds.Dy()<8||bounds.Dx()>6000||bounds.Dy()>6000{return nil,errors.New("kích thước ảnh không hợp lệ")}
 return &asset{img:img},nil
}
type newsPictureResult struct { URL string; Picture *asset; Err error }
func drawNewsPicture(fb *framebuffer,a *asset,x,y,maxW,maxH int){
 if a==nil||a.img==nil{return}
 b:=a.img.Bounds()
 w,h:=b.Dx(),b.Dy()
 if w<1||h<1{return}
 if w>maxW {h=h*maxW/w;w=maxW}
 if h>maxH {w=w*maxH/h;h=maxH}
 if w<1||h<1{return}
 x+=(maxW-w)/2
 for py:=0;py<h;py++ {
  sy:=b.Min.Y+py*b.Dy()/h
  for px:=0;px<w;px++ {
   sx:=b.Min.X+px*b.Dx()/w
   r,g,bb,alpha:=a.img.At(sx,sy).RGBA()
   if alpha<0x1000{continue}
   fb.putPixel(x+px,y+py,color{uint8(r>>8),uint8(g>>8),uint8(bb>>8)})
  }
 }
}
func newsWrappedLines(text string,maxW,scale int)[]string {
 words:=strings.Fields(text)
 result:=make([]string,0,8)
 line:=""
 for _,word:=range words {
  next:=word;if line!=""{next=line+" "+word}
  if asciiWidth(scale,next)>maxW&&line!=""{
   result=append(result,line)
   line=word
  }else{line=next}
 }
 if line!=""{result=append(result,line)}
 return result
}
func newsBlockHeight(fb *framebuffer,b newsBlock)int{
 if b.Kind!="text"{return max(125,fb.h*28/100)+25}
 lineCount:=len(newsWrappedLines(b.Text,fb.w-2*max(18,fb.w/50)-24,2))
 return max(34,lineCount*25+18)
}
func articleVisibleImage(a newsArticle,index int)string{
 if index<0{index=0}
 if index>=len(a.Blocks){return ""}
 for i:=index;i<len(a.Blocks)&&i<index+5;i++ {
  link:=newsImageForBlock(a.Blocks[i]);if link!=""{return link}
 }
 return ""
}
func articleVideoAt(a newsArticle,position int)string {
 if position>=0&&position<len(a.Blocks)&&a.Blocks[position].Kind=="video" {return a.Blocks[position].URL}
 return ""
}
func draw24hRichArticle(fb *framebuffer,page int,a newsArticle,loading bool,errText string,offset int,pictures map[string]*asset,imageErrors map[string]string,videoNote string){
 fb.fill(cBg)
 margin:=max(18,fb.w/50)
 top:=drawAppTopBar(fb,page)
 footer:=bottomTabsHeight(fb)
 drawASCII(fb,margin,top+7,1,"BÀI VIẾT 24H.COM.VN",cMuted)
 // Significantly larger and bold title, wrapped on every screen of the article.
 titleLines:=newsWrappedLines(a.Title,fb.w-2*margin-6,3)
 y:=top+31
 for i,line:=range titleLines {
  if i==4{break}
  drawASCII(fb,margin,y,3,line,cText)
  drawASCII(fb,margin+1,y,3,line,cText)
  y+=32
 }
 y+=17
 bottom:=fb.h-footer-8
 if loading {drawASCII(fb,margin,y,2,"ĐANG TẢI NỘI DUNG...",cYellow);y+=27}
 if errText!="" {drawASCII(fb,margin,y,1,cutNews(errText,85),cRed);y+=25}
 if len(a.Blocks)==0 && !loading {drawASCII(fb,margin,y,2,"NGUỒN CHƯA TRẢ NỘI DUNG BÀI",cMuted)}
 if offset<0{offset=0}
 if offset>len(a.Blocks){offset=len(a.Blocks)}
 for i:=offset;i<len(a.Blocks)&&y<bottom-16;i++{
  b:=a.Blocks[i]
  if b.Kind=="text"{
   lines:=newsWrappedLines(b.Text,fb.w-2*margin-16,2)
   for _,line:=range lines{
    if y+19>=bottom{break}
    drawASCII(fb,margin+6,y,2,line,cText)
    y+=24
   }
   y+=18
  }else{
   imageURL:=newsImageForBlock(b)
   picH:=max(110,min(220,bottom-y-35))
   if picH<40{break}
   fb.rect(margin+4,y,fb.w-2*margin-8,picH,cPanel2)
   if pic:=pictures[imageURL];pic!=nil{
    drawNewsPicture(fb,pic,margin+10,y+3,fb.w-2*margin-20,picH-6)
   }else{
    label:="ĐANG TẢI HÌNH ẢNH..."
    if imageErrors[imageURL]!=""{label="KHÔNG TẢI ĐƯỢC ẢNH"}
    if b.Kind=="video"{label="VIDEO: XEM ẢNH XEM TRƯỚC"}
    drawASCII(fb,margin+15,y+picH/2,1,label,cMuted)
   }
   if b.Kind=="video"{
    drawASCII(fb,margin+20,y+10,2,"VIDEO 24H",cYellow)
    drawASCII(fb,margin+20,y+picH-27,1,"A: PHÁT NẾU CÓ MPV VÀ LINK TRỰC TIẾP",cYellow)
   }
   y+=picH+18
   if b.Text!=""{
    drawASCII(fb,margin+7,y,1,cutNews(b.Text,95),cMuted)
    y+=20
   }
  }
 }
 if videoNote!="" {drawASCII(fb,margin,fb.h-footer-20,1,cutNews(videoNote,95),cYellow)}
 drawBottomTabs(fb,page,"LÊN/XUỐNG: CUỘN   A: VIDEO   B: TRỞ LẠI   X: TẢI LẠI")
}
func play24hVideo(target string)error{
 u,err:=url.Parse(target)
 if err!=nil||u.Scheme!="https"{return errors.New("video URL không hợp lệ")}
 host:=strings.ToLower(u.Hostname())
 if host!="24h.com.vn"&&!strings.HasSuffix(host,".24h.com.vn"){return errors.New("video nhúng cần trình duyệt, không có file video trực tiếp")}
 ext:=strings.ToLower(path.Ext(u.Path))
 if ext!=".mp4"&&ext!=".m3u8"&&ext!=".webm"{return errors.New("24h không cung cấp đường dẫn MP4/HLS trực tiếp")}
 player,err:=exec.LookPath("mpv")
 if err!=nil{return errors.New("Brick Pro cần MPV để phát video; hiện chỉ xem được ảnh preview")}
 return exec.Command(player,"--no-config","--really-quiet","--fullscreen","--",target).Run()
}
