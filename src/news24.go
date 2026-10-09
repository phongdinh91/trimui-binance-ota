package main

import (
 "errors"
 "fmt"
 "html"
 "io"
 "net/http"
 "net/url"
 "regexp"
 "strings"
 "time"
)

type newsCategory struct { Name, URL string; Gold bool }
type newsArticle struct { Title, URL, Excerpt string; Blocks []newsBlock }
type news24Result struct { Key, Kind string; Items []newsArticle; Article newsArticle; Err error; At time.Time }

// Verified 24h.com.vn category links. The gold tile uses the existing quote screen.
var businessCategories=[]newsCategory{
 {"GIÁ VÀNG","https://www.24h.com.vn/gia-vang-hom-nay-c425.html",true},
 {"TỶ GIÁ","https://www.24h.com.vn/ty-gia-ngoai-te-ttcb-c426.html",false},
 {"CHỨNG KHOÁN","https://www.24h.com.vn/tin-chung-khoan-c566.html",false},
 {"KINH TẾ THẾ GIỚI","https://www.24h.com.vn/kinh-te-the-gioi-c1106.html",false},
 {"BẤT ĐỘNG SẢN","https://www.24h.com.vn/bat-dong-san-c792.html",false},
 {"DOANH NHÂN","https://www.24h.com.vn/doanh-nhan-c606.html",false},
 {"NHÀ ĐẸP","https://www.24h.com.vn/nha-dep-c161e4825.html",false},
 {"KHỞI NGHIỆP","https://www.24h.com.vn/khoi-nghiep-c826.html",false},
 {"NGÂN HÀNG","https://www.24h.com.vn/ngan-hang-c850.html",false},
 {"QUẢN LÝ TIỀN","https://www.24h.com.vn/toi-tieu-tien-c851.html",false},
 {"DOANH NGHIỆP","https://www.24h.com.vn/doanh-nghiep-c849.html",false},
 {"THỊ TRƯỜNG","https://www.24h.com.vn/thi-truong-tieu-dung-c52.html",false},
}
var hitechCategories=[]newsCategory{
 {"HI-TECH 24H","https://www.24h.com.vn/thoi-trang-hi-tech-c407.html",false},
 {"ĐIỆN THOẠI","https://www.24h.com.vn/dien-thoai-c419.html",false},
 {"ĐÁNH GIÁ","https://www.24h.com.vn/danh-gia-san-pham-c781.html",false},
 {"LAPTOP","https://www.24h.com.vn/laptop-gia-re-c451.html",false},
 {"TIN CÔNG NGHỆ","https://www.24h.com.vn/tin-tuc-cong-nghe-c453.html",false},
 {"ĐIỆN TỬ GIA DỤNG","https://www.24h.com.vn/dien-tu-gia-dung-c786.html",false},
 {"MÁY TÍNH BẢNG","https://www.24h.com.vn/may-tinh-bang-c699.html",false},
 {"BẢNG GIÁ ĐT","https://www.24h.com.vn/bang-gia-dien-thoai-c951.html",false},
 {"IPHONE 17","https://www.24h.com.vn/iphone-17-c407e7835.html",false},
 {"GALAXY A","https://www.24h.com.vn/galaxy-a-c407e6120.html",false},
 {"IPHONE","https://www.24h.com.vn/iphone-c407e1576.html",false},
 {"SAMSUNG GALAXY","https://www.24h.com.vn/samsung-galaxy-c407e4391.html",false},
}
func newsCategories(page int) []newsCategory {
 if page==pageHitech{return hitechCategories}
 return businessCategories
}
func newsCategoryFor(page,index int)(newsCategory,bool){
 categories:=newsCategories(page)
 if index<0||index>=len(categories){return newsCategory{},false}
 return categories[index],true
}
func newsSafeURL(raw string) bool {
 u,err:=url.Parse(raw)
 if err!=nil||u.Scheme!="https"||u.User!=nil||u.Port()!=""{return false}
 return u.Hostname()=="www.24h.com.vn"||u.Hostname()=="24h.com.vn"||u.Hostname()=="amp.24h.com.vn"
}
var newsAnchors=regexp.MustCompile("(?is)<a\\b[^>]*?href\\s*=\\s*(?:\"([^\"]+)\"|'([^']+)')[^>]*>(.*?)</a\\s*>")
var newsHeadlineURL=regexp.MustCompile("(?i)-c[0-9]+(?:e[0-9]+)?a[0-9]+\\.html(?:\\?.*)?$")
var newsParagraphs=regexp.MustCompile("(?is)<p\\b[^>]*>(.*?)</p\\s*>")
var newsDescription=regexp.MustCompile("(?is)<meta\\b[^>]*name\\s*=\\s*[\"']description[\"'][^>]*content\\s*=\\s*[\"']([^\"']+)[\"']")
var newsTitleTag=regexp.MustCompile("(?is)<h1\\b[^>]*>(.*?)</h1\\s*>")
func tidyNewsText(raw string)string{
 return strings.Join(strings.Fields(goldText(raw))," ")
}
func cutNews(s string, maxRunes int) string {
 rr:=[]rune(s)
 if len(rr)<=maxRunes{return s}
 if maxRunes<4{return string(rr[:maxRunes])}
 return strings.TrimSpace(string(rr[:maxRunes-3]))+"..."
}
func parse24hArticles(page string)[]newsArticle{
 matches:=newsAnchors.FindAllStringSubmatch(page,-1)
 result:=make([]newsArticle,0,14)
 seen:=map[string]bool{}
 for _,m:=range matches {
  link:=html.UnescapeString(m[1]);if link=="" {link=html.UnescapeString(m[2])}
  if strings.HasPrefix(link,"/"){link="https://www.24h.com.vn"+link}
  if !newsSafeURL(link){continue}
  parsed,err:=url.Parse(link)
  if err!=nil||!newsHeadlineURL.MatchString(parsed.Path){continue}
  parsed.RawQuery="";parsed.Fragment=""
  link=parsed.String()
  if seen[link]{continue}
  title:=tidyNewsText(m[3])
  if len([]rune(title))<12||len([]rune(title))>260 {continue}
  seen[link]=true
  result=append(result,newsArticle{Title:title,URL:link})
  if len(result)>=14 {break}
 }
 return result
}
func parse24hArticleText(page string)newsArticle{
 out:=newsArticle{}
 if m:=newsTitleTag.FindStringSubmatch(page);len(m)>1 {out.Title=tidyNewsText(m[1])}
 if m:=newsDescription.FindStringSubmatch(page);len(m)>1 {out.Excerpt=tidyNewsText(m[1])}
 if out.Excerpt=="" {
  ps:=newsParagraphs.FindAllStringSubmatch(page,20)
  for _,p:=range ps {
   line:=tidyNewsText(p[1])
   if len([]rune(line))>70&&!strings.Contains(strings.ToLower(line),"cookie") {out.Excerpt=line;break}
  }
 }
 out.Excerpt=cutNews(out.Excerpt,420)
 return out
}
func get24hPage(client *http.Client, address string)(string,error){
 if !newsSafeURL(address){return "",errors.New("đường dẫn nguồn không hợp lệ")}
 req,err:=http.NewRequest(http.MethodGet,address,nil)
 if err!=nil{return "",err}
 req.Header.Set("User-Agent","Mozilla/5.0 (Linux; Android 12) AppleWebKit/537.36 Chrome/120 Mobile Safari/537.36")
 req.Header.Set("Accept-Language","vi-VN,vi;q=0.9")
 resp,err:=client.Do(req)
 if err!=nil{return "",err}
 defer resp.Body.Close()
 if resp.StatusCode!=200{return "",fmt.Errorf("24h HTTP %d",resp.StatusCode)}
 if ct:=resp.Header.Get("Content-Type");ct!=""&&!strings.Contains(strings.ToLower(ct),"html"){return "",errors.New("24h không trả HTML")}
 b,err:=io.ReadAll(io.LimitReader(resp.Body,3*1024*1024+1))
 if err!=nil{return "",err}
 if len(b)>3*1024*1024{return "",errors.New("trang 24h quá lớn")}
 return string(b),nil
}
func fetch24hNews(client *http.Client,address,kind string)news24Result{
 r:=news24Result{Key:address,Kind:kind,At:time.Now()}
 page,err:=get24hPage(client,address)
 if err!=nil {r.Err=err;return r}
 if kind=="article"{
  r.Article=parse24hRichArticle(page,address)
  // AMP markup sometimes exposes the body when desktop HTML hides it.
  if len(r.Article.Blocks)<2 {
   if base,e:=url.Parse(address);e==nil&&base.Hostname()!="amp.24h.com.vn"{
    base.Host="amp.24h.com.vn"
    if ampPage,e:=get24hPage(client,base.String());e==nil {
     alternative:=parse24hRichArticle(ampPage,base.String())
     if len(alternative.Blocks)>len(r.Article.Blocks){
      if alternative.Title==""{alternative.Title=r.Article.Title}
      r.Article=alternative
     }
    }
   }
  }
  if len(r.Article.Blocks)==0 {r.Err=errors.New("không đọc được nội dung từ 24h")}
  return r
 }
 r.Items=parse24hArticles(page)
 if len(r.Items)==0 {r.Err=errors.New("chưa đọc được tin mục này từ 24h")}
 return r
}
func draw24hTileGrid(fb *framebuffer,page,selected int){
 cats:=newsCategories(page)
 fb.fill(cBg)
 margin:=max(18,fb.w/50)
 headerH:=drawAppTopBar(fb,page)
 drawASCII(fb,margin,headerH+7,2,"DANH MỤC 24H.COM.VN",cText)
 drawASCII(fb,margin,headerH+34,1,fmt.Sprintf("%d MỤC CON",len(cats)),cMuted)
 footer:=bottomTabsHeight(fb)
 gap:=max(10,fb.w/90)
 rows:=4
 available:=fb.h-headerH-footer-70
 cellH:=(available-gap*(rows-1))/rows
 cellW:=(fb.w-2*margin-gap)/2
 if cellH<38 {cellH=38}
 if selected<0 {selected=0}
 if selected>=len(cats) {selected=len(cats)-1}
 row:=selected/2
 startRow:=row-1
 if startRow<0{startRow=0}
 if startRow>2{startRow=2}
 for i:=startRow*2;i<len(cats)&&i<(startRow+4)*2;i++ {
  local:=i-startRow*2
  x:=margin+(local%2)*(cellW+gap)
  y:=headerH+59+(local/2)*(cellH+gap)
  selectedTile:=i==selected
  bg:=cPanel;if selectedTile{bg=cPanel2}
  fb.rect(x,y,cellW,cellH,bg)
  if selectedTile{fb.rect(x,y,5,cellH,cYellow)}
  scale:=3
  lines:=newsWrappedLines(cats[i].Name,cellW-32,scale)
  if len(lines)>2 {scale=2;lines=newsWrappedLines(cats[i].Name,cellW-32,scale)}
  if len(lines)>2 {lines=lines[:2]}
  yy:=y+max(8,(cellH-len(lines)*(7*scale+8))/2)
  for _,line:=range lines {
   drawASCII(fb,x+13,yy,scale,line,cText)
   drawASCII(fb,x+14,yy,scale,line,cText)
   yy+=7*scale+8
  }
  small:=fmt.Sprintf("%02d/12",i+1)
  drawASCII(fb,x+cellW-asciiWidth(1,small)-10,y+8,1,small,cMuted)
 }
 drawBottomTabs(fb,page,"A: MỞ   D-PAD: CHỌN MỤC   L1/R1: ĐỔI TAB")
}
func draw24hNewsList(fb *framebuffer,page int,cat newsCategory,items []newsArticle,selected int,loading bool,errText string,updated time.Time){
 fb.fill(cBg)
 margin:=max(18,fb.w/50)
 headerH:=drawAppTopBar(fb,page)
 footer:=bottomTabsHeight(fb)
 drawASCII(fb,margin,headerH+8,3,cat.Name,cYellow)
 drawASCII(fb,margin,headerH+40,1,"NGUỒN: 24H.COM.VN",cMuted)
 if !updated.IsZero(){drawASCII(fb,margin,headerH+57,1,"TẢI LÚC "+updated.Format("15:04 02/01"),cMuted)}
 if loading{drawASCII(fb,fb.w/2,headerH+57,1,"ĐANG TẢI...",cYellow)}
 if errText!="" {drawASCII(fb,margin,headerH+78,1,"NGUỒN LỖI: "+cutNews(errText,50),cRed)}
 top:=headerH+96
 bottom:=fb.h-footer-12
 rowH:=max(50,min(75,(bottom-top)/6))
 visible:=max(1,(bottom-top)/rowH)
 start:=0
 if selected>=visible{start=selected-visible+1}
 for i:=start;i<len(items)&&i<start+visible;i++ {
  y:=top+(i-start)*rowH
  bg:=cPanel;if selected==i{bg=cPanel2}
  fb.rect(margin,y,fb.w-2*margin,rowH-5,bg)
  if selected==i{fb.rect(margin,y,4,rowH-5,cYellow)}
  txt:=cutNews(items[i].Title,max(30,(fb.w-2*margin-35)/13))
  drawASCII(fb,margin+12,y+12,1,txt,cText)
 }
 if len(items)==0 && !loading {
  msg:="CHƯA CÓ TIN / BẤM X TẢI LẠI"
  drawASCII(fb,margin,top+45,2,msg,cMuted)
 }
 drawBottomTabs(fb,page,"A: ĐỌC   B: QUAY LẠI   X: TẢI LẠI")
}
func draw24hArticlePreview(fb *framebuffer,page int,a newsArticle,loading bool,errText string){
 fb.fill(cBg)
 margin:=max(18,fb.w/50)
 headerH:=max(112,fb.h*15/100)
 fb.rect(0,0,fb.w,headerH,cPanel)
 drawASCII(fb,margin,12,3,"ĐỌC TIN 24H",cYellow)
 title:=[]rune(a.Title)
 maxPerLine:=max(28,(fb.w-2*margin)/13)
 y:=headerH+14
 for len(title)>0 && y<headerH+170 {
  n:=min(len(title),maxPerLine)
  drawASCII(fb,margin,y,2,string(title[:n]),cText)
  title=title[n:];y+=31
 }
 y+=20
 if loading {drawASCII(fb,margin,y,2,"ĐANG TẢI...",cYellow)}
 if errText!=""{drawASCII(fb,margin,y+32,1,cutNews(errText,60),cRed)}
 remaining:=[]rune(a.Excerpt)
 maxPerLine=max(28,(fb.w-2*margin)/9)
 for len(remaining)>0 && y<fb.h-bottomTabsHeight(fb)-25{
  n:=min(len(remaining),maxPerLine)
  drawASCII(fb,margin,y,1,string(remaining[:n]),cMuted)
  remaining=remaining[n:];y+=19
 }
 drawBottomTabs(fb,page,"B: DANH SÁCH   X: TẢI LẠI   TRÍCH YẾU 24H")
}
