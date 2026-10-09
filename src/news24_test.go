package main

import (
 "strings"
 "testing"
)

func Test24hCategoriesHaveRealAndSafeSources(t *testing.T){
 for _,tc:=range []struct{page int;want int}{{pageBusiness,12},{pageHitech,12}}{
  cats:=newsCategories(tc.page)
  if len(cats)!=tc.want {t.Fatalf("page=%d: got %d categories",tc.page,len(cats))}
  names:=map[string]bool{}
  for _,c:=range cats {
   if c.Name==""||names[c.Name] {t.Fatalf("invalid or duplicate category %+v",c)}
   if !newsSafeURL(c.URL) {t.Fatalf("unsafe category URL %s",c.URL)}
   names[c.Name]=true
  }
 }
 if !businessCategories[0].Gold||hitechCategories[0].Gold {t.Fatal("gold price must be business-only")}
 if _,ok:=newsCategoryFor(pageBusiness,12);ok {t.Fatal("out of range category accepted")}
}
func Test24hHeadlineParser(t *testing.T){
 page:=`<html><a href="https://www.24h.com.vn/kinh-doanh/gia-vang-tang-c161a12345.html">
 <h3>Giá vàng hôm nay tăng mạnh, nhà đầu tư chú ý biến động</h3></a>
 <a href="/tin-tuc-cong-nghe/dien-thoai-galaxy-c453a4533.html">Mẫu điện thoại mới có giá bán hấp dẫn</a>
 <a href="https://evil.24h.com.vn/kinh-doanh/malware-c161a99999.html">Tin giả dài thật dài</a>
 <a href="https://www.24h.com.vn/kinh-doanh/gia-vang-tang-c161a12345.html">Tin trùng lặp</a></html>`
 got:=parse24hArticles(page)
 if len(got)!=2 {t.Fatalf("got %d headlines: %+v",len(got),got)}
 if !strings.Contains(got[0].Title,"Giá vàng") {t.Fatalf("bad article title %+v",got[0])}
 if !strings.HasPrefix(got[1].URL,"https://www.24h.com.vn/"){t.Fatal("relative href not resolved")}
}
func Test24hUnsafeLinksRejected(t *testing.T){
 bad:=[]string{"https://24h.com.vn.evil.test/a","http://www.24h.com.vn/x","javascript:alert(1)","https://user:pass@www.24h.com.vn/x","https://www.24h.com.vn:444/x"}
 for _,s:=range bad{if newsSafeURL(s){t.Fatalf("accepted unsafe %q",s)}}
}
func Test24hArticleExcerpt(t *testing.T){
 html:=`<html><h1>Bản tin công nghệ mới nhất</h1>
 <meta name="description" content="Giới thiệu sản phẩm mới &amp; đánh giá xu hướng mới ở thị trường Việt Nam."></html>`
 got:=parse24hArticleText(html)
 if got.Title!="Bản tin công nghệ mới nhất" {t.Fatalf("title %q",got.Title)}
 if !strings.Contains(got.Excerpt,"& đánh giá") {t.Fatalf("excerpt %q",got.Excerpt)}
}
