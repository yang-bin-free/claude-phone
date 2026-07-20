package desktop

import (
	"bytes"
	"encoding/xml"
)

const LaunchAgentLabel = "com.codeafar.mac"

func launchAgentXML(executable string, args []string) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString(xml.Header)
	out.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	out.WriteString("<plist version=\"1.0\"><dict><key>Label</key><string>")
	if err := xml.EscapeText(&out, []byte(LaunchAgentLabel)); err != nil {
		return nil, err
	}
	out.WriteString("</string><key>ProgramArguments</key><array>")
	for _, value := range append([]string{executable}, args...) {
		out.WriteString("<string>")
		if err := xml.EscapeText(&out, []byte(value)); err != nil {
			return nil, err
		}
		out.WriteString("</string>")
	}
	out.WriteString("</array><key>RunAtLoad</key><true/><key>KeepAlive</key><false/></dict></plist>\n")
	return out.Bytes(), nil
}
