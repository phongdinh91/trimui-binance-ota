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
 if settingsNextLED(ledSystem,-1)!=ledBreathing {t.Fatal("LED backwards wrap")}
 if settingsNextLED(ledBreathing,1)!=ledSystem {t.Fatal("LED forward wrap")}
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
