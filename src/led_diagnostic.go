package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LED diagnostics are read-only: no sysfs nodes are changed here.
// This snapshot is written into the app folder so a failed runtime LED
// request can be diagnosed on the actual TG4040 without guessing firmware
// zone names or assuming a 23-pixel layout.
func recordLEDDiagnostic(status string) {
	var lines []string
	lines=append(lines,"BINANCE LED diagnostics "+time.Now().Format(time.RFC3339))
	lines=append(lines,"Status: "+status)
	if b,e:=os.ReadFile("/proc/device-tree/model");e==nil{
		lines=append(lines,"Model: "+strings.ReplaceAll(strings.TrimSpace(string(b)),"\x00"," "))
	}
	dir:=ledDriverPath("")
	nodes,e:=os.ReadDir(dir)
	if e!=nil{
		lines=append(lines,"Driver: "+e.Error())
	}else{
		var names []string
		for _,node:=range nodes{names=append(names,node.Name())}
		sort.Strings(names)
		lines=append(lines,"Nodes: "+strings.Join(names,", "))
		for _,name:=range []string{"enable","effect_enable","max_scale","max_scale_lr","max_scale_f1f2","max_scale_rear","effect_names"}{
			b,err:=os.ReadFile(filepath.Join(dir,name))
			if err!=nil{
				lines=append(lines,fmt.Sprintf("%s: unavailable (%v)",name,err))
			}else{
				value:=strings.TrimSpace(string(b))
				if len(value)>2000{value=value[:2000]}
				lines=append(lines,name+": "+value)
			}
		}
	}
	path:=filepath.Join(appDir(),"led-diagnostic.txt")
	if err:=os.WriteFile(path,[]byte(strings.Join(lines,"\n")+"\n"),0644);err!=nil{
		fmt.Fprintln(os.Stderr,"LED diagnostics:",err)
	}
}
