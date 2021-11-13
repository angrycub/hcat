package dependency

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/hcat/dep"
)

func TestVaultRenewDuration(t *testing.T) {
	renewable := dep.Secret{LeaseDuration: 100, Renewable: true}
	renewableDur := leaseCheckWait(&renewable).Seconds()
	if renewableDur < 16 || renewableDur >= 34 {
		t.Fatalf("renewable duration is not within 1/6 to 1/3 of lease duration: %f", renewableDur)
	}

	nonRenewable := dep.Secret{LeaseDuration: 100}
	nonRenewableDur := leaseCheckWait(&nonRenewable).Seconds()
	if nonRenewableDur < 85 || nonRenewableDur > 95 {
		t.Fatalf("renewable duration is not within 85%% to 95%% of lease duration: %f", nonRenewableDur)
	}

	var data = map[string]interface{}{
		"rotation_period": json.Number("60"),
		"ttl":             json.Number("30"),
	}

	nonRenewableRotated := dep.Secret{LeaseDuration: 100, Data: data}
	nonRenewableRotatedDur := leaseCheckWait(&nonRenewableRotated).Seconds()

	// We expect a 1 second cushion
	if nonRenewableRotatedDur != 31 {
		t.Fatalf("renewable duration is not 31: %f", nonRenewableRotatedDur)
	}

	data = map[string]interface{}{
		"rotation_period": json.Number("30"),
		"ttl":             json.Number("5"),
	}

	nonRenewableRotated = dep.Secret{LeaseDuration: 100, Data: data}
	nonRenewableRotatedDur = leaseCheckWait(&nonRenewableRotated).Seconds()

	// We expect a 1 second cushion
	if nonRenewableRotatedDur != 6 {
		t.Fatalf("renewable duration is not 6: %f", nonRenewableRotatedDur)
	}

	rawExpiration := time.Now().Unix() + 100
	expiration := strconv.FormatInt(rawExpiration, 10)

	data = map[string]interface{}{
		"expiration":  json.Number(expiration),
		"certificate": "foobar",
	}

	nonRenewableCert := dep.Secret{LeaseDuration: 100, Data: data}
	nonRenewableCertDur := leaseCheckWait(&nonRenewableCert).Seconds()
	if nonRenewableCertDur < 85 || nonRenewableCertDur > 95 {
		t.Fatalf("non-renewable certicate duration is not within 85%% to 95%%: %f",
			nonRenewableCertDur)
	}

	t.Run("secret ID handling", func(t *testing.T) {
		t.Run("normal case", func(t *testing.T) {
			// Secret ID TTL handling
			data := map[string]interface{}{
				"secret_id":     "abc",
				"secret_id_ttl": json.Number("60"),
			}

			secret := dep.Secret{LeaseDuration: 100, Data: data}
			secretDur := leaseCheckWait(&secret).Seconds()

			if secretDur < 0.85*(60+1) || secretDur > 0.95*(60+1) {
				t.Fatalf("renewable duration is not within 85%% to 95%% of lease duration: %f", secretDur)
			}
		})

		t.Run("0 ttl", func(t *testing.T) {
			const leaseDur = 1000

			data := map[string]interface{}{
				"secret_id":     "abc",
				"secret_id_ttl": json.Number("0"),
			}

			secret := dep.Secret{LeaseDuration: leaseDur, Data: data}
			secretDur := leaseCheckWait(&secret).Seconds()

			if secretDur < 0.85*(leaseDur+1) || secretDur > 0.95*(leaseDur+1) {
				t.Fatalf("renewable duration is not within 85%% to 95%% of lease duration: %f", secretDur)
			}
		})

		t.Run("ttl missing", func(t *testing.T) {
			const leaseDur = 1000

			data := map[string]interface{}{
				"secret_id": "abc",
			}

			secret := dep.Secret{LeaseDuration: leaseDur, Data: data}
			secretDur := leaseCheckWait(&secret).Seconds()

			if secretDur < 0.85*(leaseDur+1) || secretDur > 0.95*(leaseDur+1) {
				t.Fatalf("renewable duration is not within 85%% to 95%% of lease duration: %f", secretDur)
			}
		})

	})
}
