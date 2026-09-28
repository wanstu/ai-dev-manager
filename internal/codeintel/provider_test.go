package codeintel

import "testing"

func TestStaticProviderPublishesCurrentContract(t *testing.T) {
	info := NewStaticIndexProvider().Info()
	if info.Contract != ContractName {
		t.Fatalf("contract=%q want=%q", info.Contract, ContractName)
	}
	if info.ProtocolVersion != ContractProtocolVersion {
		t.Fatalf("protocol_version=%d want=%d", info.ProtocolVersion, ContractProtocolVersion)
	}
	if info.ID != "adm_static_index" {
		t.Fatalf("provider_id=%q", info.ID)
	}
	if !info.Capabilities.Definitions || info.Capabilities.References || info.Capabilities.Hierarchy {
		t.Fatalf("capabilities=%+v", info.Capabilities)
	}
}
