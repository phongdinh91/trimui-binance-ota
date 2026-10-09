package main

import (
 "image"
 "image/color"
 "image/png"
 "bytes"
 "io"
 "net/http"
 "strings"
 "testing"
)

func Test24hArticleContentAndMedia(t *testing.T) {
 html := `<html>
 <head><meta property="og:image" content="https://cdn.24h.com.vn/upload/cover.jpg"></head>
 <body><h1>Tiêu đề bài viết có nhiều thông tin</h1>
 <div class="text-conent">
 <p>Đây là đoạn nội dung đầu tiên của bài báo có đầy đủ thông tin và các chi tiết cần đọc.</p>
 <p><img data-original="https://cdn.24h.com.vn/upload/a.jpg" alt="Hình hiện trường"></p>
 <p>Đoạn thứ hai trong bài viết giải thích rõ ràng hơn về câu chuyện này cho người đọc.</p>
 <video poster="https://cdn.24h.com.vn/upload/preview.jpg" src="https://cdn.24h.com.vn/video/clip.mp4"></video>
 </div></body></html>`
 a:=parse24hRichArticle(html,"https://www.24h.com.vn/article-c453a777.html")
 if a.Title!="Tiêu đề bài viết có nhiều thông tin" {t.Fatalf("title=%q",a.Title)}
 if len(a.Blocks)<4{t.Fatalf("expected rich article with content/images/video, got %+v",a.Blocks)}
 if a.Blocks[0].Kind!="text"||!strings.Contains(a.Blocks[0].Text,"nội dung đầu tiên"){t.Fatalf("body paragraph %+v",a.Blocks[0])}
 images,videos:=0,0
 for _,b:=range a.Blocks {
  if b.Kind=="image"{images++;if b.URL==""{t.Fatal("missing image URL")}}
  if b.Kind=="video"{videos++;if !strings.HasSuffix(b.URL,"clip.mp4"){t.Fatalf("bad video %+v",b)}}
 }
 if images<1||videos<1 {t.Fatalf("missing media: images=%d videos=%d",images,videos)}
}
func Test24hMediaSafeAndMetadataFallback(t *testing.T){
 if newsResolveMedia("https://www.24h.com.vn/a.html","")!=""{t.Fatal("empty media URL is not valid")}
 if newsResolveMedia("https://www.24h.com.vn/a.html","https://169.254.1.5/secret")!=""{t.Fatal("private media URL accepted")}
 if newsResolveMedia("https://www.24h.com.vn/a.html","https://evil.example/img.jpg")!=""{t.Fatal("external media URL accepted")}
 a:=parse24hRichArticle(`<h1>Kiểm tra nguồn media</h1>
 <meta property="og:image" content="https://cdn.24h.com.vn/pic.jpg">
 <p>Thông tin trong nội dung có nhiều chi tiết giá trị, cần được tải về để đọc.</p>`,"https://www.24h.com.vn/hello.html")
 found:=false
 for _,b:=range a.Blocks{if b.Kind=="image"&&b.URL=="https://cdn.24h.com.vn/pic.jpg"{found=true}}
 if !found{t.Fatalf("expected metadata image, got %+v",a.Blocks)}
}
type articleTransport func(*http.Request)(*http.Response,error)
func(f articleTransport)RoundTrip(r *http.Request)(*http.Response,error){return f(r)}
func Test24hImageLoaderPNG(t *testing.T){
 img:=image.NewRGBA(image.Rect(0,0,18,16))
 img.Set(0,0,color.RGBA{255,0,0,255})
 var b bytes.Buffer
 if e:=png.Encode(&b,img);e!=nil{t.Fatal(e)}
 client:=&http.Client{Transport:articleTransport(func(r *http.Request)(*http.Response,error){
  return &http.Response{StatusCode:200,Body:io.NopCloser(bytes.NewReader(b.Bytes())),Header:http.Header{"Content-Type":[]string{"image/png"}}},nil
 })}
 a,e:=fetchNewsPicture(client,"https://cdn.24h.com.vn/upload/photo.png")
 if e!=nil{t.Fatal(e)}
 if a.img.Bounds().Dx()!=18{t.Fatal("bad loaded image")}
}
func Test24hHeaderHintSpacing(t *testing.T) {
 parts:=footerParts("A: NHẬP Y: ĐỔI VÙNG START: ẨN SELECT: YÊU THÍCH")
 if len(parts)!=4{t.Fatalf("got %v",parts)}
 if parts[0]!="A: NHẬP"||parts[1]!="Y: ĐỔI VÙNG"||parts[2]!="START: ẨN"||parts[3]!="SELECT: YÊU THÍCH"{t.Fatalf("unexpected hints %v",parts)}
}
