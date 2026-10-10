package main

import "html"

// iptvMobileReportPage is a self-contained LAN page. It makes no third-party
// requests and works without HTTPS, which Stock OS's local HTTP server lacks.
// Clipboard and Web Share permissions vary by mobile browser: preserve a
// selectable report and offer a client-side .txt file as fallbacks.
func iptvMobileReportPage(report string) string {
 const top = `<!doctype html>
<html lang="vi"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="referrer" content="no-referrer">
<title>Báo cáo IPTV – BINANCE</title>
<style>
*{box-sizing:border-box}body{font:16px/1.45 system-ui,-apple-system,sans-serif;margin:0;background:#101827;color:#f3f4f6}
main{max-width:820px;margin:auto;padding:18px}
h1{font-size:23px;margin:8px 0}p{line-height:1.5}
.actions{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin:16px 0}
button{border:0;border-radius:10px;background:#f2c230;color:#111827;font:600 16px system-ui,sans-serif;min-height:50px;padding:12px}
button.secondary{background:#e5e7eb}button:active{opacity:.7}
#status{padding:8px 0;min-height:48px;color:#facc15;font-weight:600}
textarea{display:block;width:100%;height:52vh;min-height:230px;padding:14px;border-radius:9px;background:#fff;color:#111827;font:13px/1.5 ui-monospace,SFMono-Regular,Consolas,monospace;resize:vertical;user-select:text;-webkit-user-select:text}
small{display:block;color:#d1d5db;margin:15px 0}
</style></head>
<body><main>
<h1>Báo cáo chẩn đoán BINANCE IPTV</h1>
<p>Bạn có thể sao chép để dán vào ChatGPT, chia sẻ bằng điện thoại, hoặc tải tệp <strong>iptv-diagnostic.txt</strong>. Không tự động gửi báo cáo ra Internet.</p>
<div class="actions">
<button id="copy" type="button">Sao chép báo cáo</button>
<button id="share" class="secondary" type="button">Chia sẻ</button>
<button id="download" class="secondary" type="button">Tải tệp .txt</button>
<button id="select" class="secondary" type="button">Chọn toàn bộ</button>
</div>
<div id="status" aria-live="polite">Nếu nút Sao chép không hoạt động, hãy dùng Tải tệp .txt hoặc Chọn toàn bộ.</div>
<textarea id="report" readonly spellcheck="false">`
 const bottom = `</textarea>
<small>Trình duyệt có thể hạn chế quyền sao chép hoặc chia sẻ trên trang HTTP nội bộ. Khi đó, hãy tải tệp .txt hoặc chạm giữ đoạn chữ đã chọn.</small>
</main><script>
(function(){
  "use strict";
  var report=document.getElementById("report");
  var status=document.getElementById("status");
  function say(s){status.textContent=s;}
  function selectAll(){
    report.focus();
    report.select();
    if(report.setSelectionRange){report.setSelectionRange(0,report.value.length);}
  }
  function legacyCopy(){
    // A temporary, writable textarea works on more HTTP/mobile browsers
    // than copying from a readonly field.
    var field=document.createElement("textarea");
    field.value=report.value;
    field.setAttribute("aria-hidden","true");
    field.style.cssText="position:fixed;left:0;top:0;width:1px;height:1px;opacity:0;z-index:-1";
    document.body.appendChild(field);
    field.focus();
    field.select();
    if(field.setSelectionRange){field.setSelectionRange(0,field.value.length);}
    var ok=false;
    try{ok=document.execCommand("copy");}catch(e){}
    document.body.removeChild(field);
    return ok;
  }
  document.getElementById("copy").addEventListener("click",async function(){
    // navigator.clipboard is usually unavailable on HTTP LAN origins.
    if(navigator.clipboard && window.isSecureContext){
      try{
        await navigator.clipboard.writeText(report.value);
        say("Đã sao chép! Mở ChatGPT và chọn Dán.");return;
      }catch(e){}
    }
    if(legacyCopy()){
      say("Đã sao chép! Mở ChatGPT và chọn Dán.");
    }else{
      selectAll();
      say("Trình duyệt chặn sao chép tự động. Chạm giữ phần đã chọn để Sao chép hoặc chọn Tải tệp .txt.");
    }
  });
  document.getElementById("select").addEventListener("click",function(){
    selectAll();
    say("Đã chọn toàn bộ. Chạm giữ đoạn chữ và nhấn Sao chép.");
  });
  document.getElementById("download").addEventListener("click",function(){
    try{
      var blob=new Blob([report.value],{type:"text/plain;charset=utf-8"});
      var url=URL.createObjectURL(blob);
      var link=document.createElement("a");
      link.href=url;
      link.download="iptv-diagnostic.txt";
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      setTimeout(function(){URL.revokeObjectURL(url);},5000);
      say("Đã yêu cầu tải iptv-diagnostic.txt. Mở Tệp/Tải về trên điện thoại, rồi đính kèm vào ChatGPT.");
    }catch(e){
      selectAll();
      say("Không tải được tệp trên trình duyệt này. Hãy chọn và sao chép nội dung.");
    }
  });
  document.getElementById("share").addEventListener("click",async function(){
    if(navigator.share){
      try{
        await navigator.share({title:"Báo cáo BINANCE IPTV",text:report.value});
        say("Đã mở bảng chia sẻ của điện thoại.");return;
      }catch(e){
        if(e&&e.name==="AbortError"){say("Đã hủy chia sẻ.");return;}
      }
    }
    selectAll();
    say("Trình duyệt không hỗ trợ chia sẻ trực tiếp tại HTTP nội bộ. Chọn Tải tệp .txt hoặc Sao chép.");
  });
})();
</script></body></html>`
 return top+html.EscapeString(report)+bottom
}
