package main

import (
 "strings"
 "testing"
)

func TestIPTVFootballFilterDoesNotPretendToCarryPremierLeague(t *testing.T){
 all:=[]iptvChannel{
  {ID:"news",Name:"VTV1 Tin Tức",URLs:[]string{"https://tv.test/news.m3u8"}},
  {ID:"sport",Name:"HTV Thể Thao",URLs:[]string{"https://tv.test/sport.m3u8"}},
  {ID:"football",Name:"Bóng Đá Việt Nam",URLs:[]string{"https://tv.test/soccer.m3u8"}},
  {ID:"music",Name:"Music TV",URLs:[]string{"https://tv.test/music.m3u8"}},
  {ID:"web",Name:"Premier League NEWS",URLs:nil},
 }
 sport:=iptvVisibleChannels(all,iptvSectionFootball)
 if len(sport)!=3{t.Fatalf("expected FPT official guide and two playable sports channels, got %+v",sport)}
 if !iptvIsOfficialEPLEntry(sport[0]) {t.Fatalf("first item should be FPT Play official portal: %+v",sport[0])}
 if sport[0].URLs!=nil {t.Fatal("an FPT website must not be misrepresented as an MPV stream")}
 if sport[1].ID!="sport"||sport[2].ID!="football" {t.Fatalf("wrong order/filter: %+v",sport)}
 full:=iptvVisibleChannels(all,iptvSectionAll)
 if len(full)!=len(all)||full[0].ID!="news"{t.Fatal("all category changed")}
 if iptvIsFootballChannel("VTV1 Tin Tức"){t.Fatal("news should not be in sports")}
}
func TestIPTVFootballAlwaysHasOfficialGuideEvenWithNoStreams(t *testing.T){
 sport:=iptvVisibleChannels(nil,iptvSectionFootball)
 if len(sport)!=1||!iptvIsOfficialEPLEntry(sport[0]) {
  t.Fatalf("missing official guide with empty playlist: %+v",sport)
 }
 if !strings.HasPrefix(iptvOfficialEPLURL,"https://fptplay.vn/") {
  t.Fatal("official QR must point to verified official FPT Play origin")
 }
 if iptvAcceptURL(iptvOfficialEPLURL)==false{t.Fatal("official portal should be a valid HTTPS link")}
}
func TestIPTVOfficialPremierLeagueQRIsValid(t *testing.T){
 qr:=iptvOfficialQR()
 if len(qr)<21||len(qr)>60{t.Fatalf("unexpected QR size %d",len(qr))}
 for _,r:=range qr {if len(r)!=len(qr){t.Fatalf("non-square QR bitmap")}}
 count:=0
 for _,r:=range qr {for _,dark:=range r {if dark{count++}}}
 if count<100{t.Fatalf("empty QR bitmap")}
}
func TestIPTVFootballDoesNotAlterPlaybackFallbackOrURLs(t *testing.T){
 all:=[]iptvChannel{{ID:"sport-a",Name:"ON Football",URLs:[]string{
   "https://provider.test/a.m3u8","https://provider.test/backup.m3u8",
 }}}
 sport:=iptvVisibleChannels(all,iptvSectionFootball)
 if len(sport)!=2 ||len(sport[1].URLs)!=2 {
  t.Fatalf("filter broke authorized backups: %+v",sport)
 }
 if !iptvIsOfficialEPLEntry(sport[0]) || iptvIsOfficialEPLEntry(sport[1]) {
  t.Fatal("real streams must stay distinct from official information card")
 }
}
