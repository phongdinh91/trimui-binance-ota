package main

import (
 "fmt"
 "strings"
 "testing"
)

func TestLEDZoneMustTriggerAfterRGB(t *testing.T) {
 var writes []string
 record:=func(name,value string)error{
  writes=append(writes,name+"="+value)
  return nil
 }
 if err:=ledStudioApplyZone("rear","10A0FF",record);err!=nil{t.Fatal(err)}
 want:=[]string{
  "effect_rgb_hex_rear=10A0FF ",
  "effect_rear=0",
  "effect_rear=4",
 }
 if len(writes)!=len(want){t.Fatalf("writes=%q; want=%q",writes,want)}
 for i:=range want{
  if writes[i]!=want[i]{t.Fatalf("write %d: got %q, want %q",i,writes[i],want[i])}
 }
}
func TestLEDZoneTriggerStopsOnWriteError(t *testing.T){
 var writes []string
 record:=func(name,value string)error{
  writes=append(writes,name)
  if name=="effect_l" {return fmt.Errorf("permission denied")}
  return nil
 }
 err:=ledStudioApplyZone("l","FFFFFF",record)
 if err==nil||!strings.Contains(err.Error(),"permission denied"){t.Fatalf("error must propagate: %v",err)}
 if len(writes)!=2{t.Fatalf("expected short circuit, got %q",writes)}
}
func TestLEDStudioRejectsMalformedRGB(t *testing.T){
 if err:=ledStudioApplyZone("l","FF",func(string,string)error{return nil});err==nil{
  t.Fatal("invalid RGB must not be sent to kernel")
 }
}
