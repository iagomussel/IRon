package ir

import "testing"

func TestPacket_Validate(t *testing.T) {
	tests := []struct {
		name    string
		packet  Packet
		wantErr bool
	}{
		{
			name: "valid packet",
			packet: Packet{
				Action: ActionActNow,
				Risk:   RiskLow,
			},
			wantErr: false,
		},
		{
			name: "invalid action",
			packet: Packet{
				Action: "invalid",
				Risk:   RiskLow,
			},
			wantErr: true,
		},
		{
			name: "invalid risk",
			packet: Packet{
				Action: ActionSchedule,
				Risk:   "extreme",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.packet.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Packet.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
