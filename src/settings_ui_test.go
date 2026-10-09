package main

import (
 "strings"
 "testing"
)

func TestSettingsDefaultAndNormalization(t *testing.T){
 s:=normalizeSettings(settingsFile{LEDMode:ledModeCount+5,ThemeLight:true,RefreshIndex:defaultRefreshIndex})
 if s.LEDMode!=ledSystem {t.Fatalf("invalid LED mode must reset to SYSTEM, got %d",s.LEDMode)}
 if !s.ThemeLight {t.Fatal("light theme must be preserved")}
}
func TestSettingsLEDNavigation(t *testing.T){
 if settingsNextLED(ledSystem,-1)!=ledGreen {t.Fatal("LED backwards wrap")}
 if settingsNextLED(ledGreen,1)!=ledSystem {t.Fatal("LED forward wrap")}
 if settingsNextLED(ledOff,1)!=ledGold {t.Fatal("LED next")}
}
func TestSettingsThemeColors(t *testing.T){
 applyTheme(true)
 if cBg.r<200||cText.r>80 {t.Fatalf("expected light palette bg=%+v text=%+v",cBg,cText)}
 applyTheme(false)
 if cBg.r>60||cText.r<200 {t.Fatalf("expected restored dark palette bg=%+v text=%+v",cBg,cText)}
}
func TestSettingsLabelAndToggle(t *testing.T){
 s:=normalizeSettings(settingsFile{ThemeLight:true,LEDMode:ledCyan})
 if settingsValue(s,settingsTheme)!="SÁNG" {t.Fatal("light switch label")}
 if !strings.Contains(settingsValue(s,settingsLED),"XANH") {t.Fatal("LED mode label")}
 if settingsValue(s,settingsOTA)=="" {t.Fatal("OTA action must be visible")}
}
func TestLEDBreathEffectDetection(t *testing.T){
 if id,ok:=ledBreathEffectID("0: off\n3: Breathe\n4: solid");!ok||id!="3" {t.Fatalf("expected breath effect 3, got %q %t",id,ok)}
 if _,ok:=ledBreathEffectID("0: off\n4: static");ok {t.Fatal("must not invent effect number")}
}
func TestOTAManualOnlyDesign(t *testing.T){
 // OTA startup switch in previous installers does not change Settings defaults.
 // Manual version check is triggered only by the settingsOTA action in main.
 s:=normalizeSettings(settingsFile{})
 if s.ThemeLight||s.LEDMode!=ledSystem {t.Fatal("new Settings must be opt-in")}
}

func TestLEDBrightnessAndSpeedBounds(t *testing.T){
 if settingsNextBrightness(40,1)!=50 || settingsNextBrightness(10,-1)!=10 || settingsNextBrightness(60,1)!=60 {t.Fatal("Brightness bounds")}
 if settingsNextSpeed(3,-1)!=2 || settingsNextSpeed(1,-1)!=1 || settingsNextSpeed(5,1)!=5 {t.Fatal("Speed bounds")}
 for i:=1;i<=5;i++{
  if ledDurationMS[i]<150||ledDurationMS[i]>1800 {t.Fatalf("unexpected duration %d",ledDurationMS[i])}
  if i>1 && ledDurationMS[i]<=ledDurationMS[i-1] {t.Fatal("Speed durations not ordered")}
 }
}
func TestNativeLEDEffectNames(t *testing.T){
 text:="0: off\n3: Breathe\n4: Static\n5: Blink\n6: Rainbow\n7: Chase"
 cases:=[]struct{mode int;expected string}{{ledBreathing,"3"},{ledBlink,"5"},{ledRainbow,"6"},{ledChase,"7"}}
 for _,tc:=range cases{
  id,ok:=ledNativeEffectID(text,tc.mode)
  if !ok||id!=tc.expected {t.Fatalf("mode %d: %s %t",tc.mode,id,ok)}
 }
 if _,ok:=ledNativeEffectID("4: Static",ledRainbow);ok {t.Fatal("Should reject unsupported native effects")}
}
func TestLEDSettingsNormalizationDefaults(t *testing.T){
 s:=normalizeSettings(settingsFile{})
 if s.LEDBrightness!=ledBrightnessDefault || s.LEDSpeed!=ledSpeedDefault {t.Fatalf("Old settings require defaults: %+v",s)}
 s=normalizeSettings(settingsFile{LEDBrightness:100,LEDSpeed:-1})
 if s.LEDBrightness!=ledBrightnessDefault||s.LEDSpeed!=ledSpeedDefault {t.Fatal("Out of range controls not normalized")}
}
func TestLEDSubmenuRows(t *testing.T){
 s:=normalizeSettings(settingsFile{})
 if settingsItemCount!=4 || ledSubItemCount!=3 || ledSubEffect==ledSubBrightness || ledSubBrightness==ledSubSpeed {
  t.Fatal("LED settings must be nested under one main Settings row")
 }
 if settingsValue(s,settingsLED)=="" || settingsValue(s,settingsAbout)==""{t.Fatal("Missing Settings labels")}
}
