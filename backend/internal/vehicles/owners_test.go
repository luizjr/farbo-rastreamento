package vehicles

import (
	"testing"

	"github.com/google/uuid"
)

func TestOwnerIndexOwns(t *testing.T) {
	alice, bob := uuid.New(), uuid.New()
	aliceVehicle, aliceDevice := uuid.New(), uuid.New()
	centralVehicle := uuid.New()

	index := &OwnerIndex{
		byVehicle: map[uuid.UUID]uuid.UUID{aliceVehicle: alice},
		byDevice:  map[uuid.UUID]uuid.UUID{aliceDevice: alice},
	}

	cases := []struct {
		name     string
		customer uuid.UUID
		vehicle  *uuid.UUID
		device   *uuid.UUID
		want     bool
	}{
		{"dona pelo veículo", alice, &aliceVehicle, nil, true},
		{"dona pelo rastreador", alice, nil, &aliceDevice, true},
		{"outro cliente, mesmo veículo", bob, &aliceVehicle, nil, false},
		{"outro cliente, mesmo rastreador", bob, nil, &aliceDevice, false},
		{"veículo da central (sem dono)", alice, &centralVehicle, nil, false},
		{"mensagem sem veículo nem rastreador", alice, nil, nil, false},
		// O veículo manda: não adianta o rastreador ser da cliente se o
		// veículo informado não é.
		{"veículo alheio com rastreador próprio", alice, &centralVehicle, &aliceDevice, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := index.Owns(tc.customer, tc.vehicle, tc.device); got != tc.want {
				t.Fatalf("Owns = %v, esperado %v", got, tc.want)
			}
		})
	}
}
