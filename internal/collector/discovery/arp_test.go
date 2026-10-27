package discovery

import "testing"

func TestParseARP(t *testing.T) {
	tests := []struct {
		name  string
		parse func(string) map[string]string
		out   string
	}{
		{"darwin", wrap(parseDarwinARP), `? (192.168.0.1) at 22:6a:94:0:1:2 on en0 ifscope [ethernet]
? (192.168.0.186) at 6:c7:ef:a:b:c on en0 ifscope [ethernet]
? (192.168.0.9) at (incomplete) on en0 ifscope [ethernet]
? (192.168.0.255) at ff:ff:ff:ff:ff:ff on en0 ifscope [ethernet]
? (224.0.0.251) at 1:0:5e:0:0:fb on en0 ifscope permanent [ethernet]
`},
		{"linux", wrap(parseLinuxARP), `IP address       HW type     Flags       HW address            Mask     Device
192.168.0.1      0x1         0x2         22:6a:94:00:01:02     *        eth0
192.168.0.186    0x1         0x2         06:c7:ef:0a:0b:0c     *        eth0
192.168.0.9      0x1         0x0         00:00:00:00:00:00     *        eth0
`},
		{"windows", wrap(parseWindowsARP), `
Interface: 192.168.0.155 --- 0x7
  Internet Address      Physical Address      Type
  192.168.0.1           22-6a-94-00-01-02     dynamic
  192.168.0.186         06-c7-ef-0a-0b-0c     dynamic
  192.168.0.255         ff-ff-ff-ff-ff-ff     static
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.parse(tt.out)
			want := map[string]string{"192.168.0.1": "22:6a:94:00:01:02", "192.168.0.186": "06:c7:ef:0a:0b:0c"}
			if len(got) != len(want) {
				t.Fatalf("got %v, want %v", got, want)
			}
			for ip, mac := range want {
				if got[ip] != mac {
					t.Errorf("%s = %q, want %q", ip, got[ip], mac)
				}
			}
		})
	}
}

func wrap[M ~map[string]V, V interface{ String() string }](parse func(string) M) func(string) map[string]string {
	return func(s string) map[string]string {
		out := map[string]string{}
		for k, v := range parse(s) {
			out[k] = v.String()
		}
		return out
	}
}
