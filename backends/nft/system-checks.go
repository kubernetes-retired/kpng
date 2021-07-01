package nft

import (
	"bytes"
	"errors"
	"os"
	"os/exec"

	"k8s.io/klog"
)

func checkMapIndexBug() {
	if *forceNFTHashBug {
		hasNFTHashBug = true
		return
	}

	klog.Info("checking for NFT hash bug")

	// check the nft vmap bug (0.9.5 but protect against the whole class)
	nft := exec.Command("nft", "-f", "-")
	nft.Stdin = bytes.NewBuffer([]byte(`
table ip k8s_test_vmap_bug
delete table ip k8s_test_vmap_bug
table ip k8s_test_vmap_bug {
  map m1 {
    typeof numgen random mod 2 : ip daddr
    elements = { 1 : 10.0.0.1, 2 : 10.0.0.2 }
  }
}
`))
	if err := nft.Run(); err != nil {
		klog.Warning("failed to test nft bugs: ", err)
	}

	// cleanup on return
	defer func() {
		nft = exec.Command("nft", "-f", "-")
		nft.Stdin = bytes.NewBuffer([]byte(`
delete table ip k8s_test_vmap_bug
`))
		nft.Stdout = os.Stdout
		nft.Stderr = os.Stderr
		err := nft.Run()
		if err != nil {
			klog.Warning("failed to delete test table k8s_test_vmap_bug: ", err)
		}
	}()

	// get the recorded map
	nft = exec.Command("nft", "list", "map", "ip", "k8s_test_vmap_bug", "m1")
	output, err := nft.Output()
	if err != nil {
		klog.Warning("failed to test nft bugs: ", err)
		return
	}

	if len(bytes.TrimSpace(output)) == 0 {
		klog.Warning(`!!! WARNING !!! NFT is blind, can't auto-detect hash bug; to manually check:
-> run in this container:
# nft -f - <<EOF
table ip k8s_test_vmap_bug
delete table ip k8s_test_vmap_bug
table ip k8s_test_vmap_bug {
  map m1 {
    typeof numgen random mod 2 : ip daddr
    elements = { 1 : 10.0.0.1, 2 : 10.0.0.2 }
  }
}
EOF

-> run on your system:
# nft list map ip k8s_test_vmap_bug m1

-> if the output map is { 16777216 : 10.0.0.1, 33554432 : 10.0.0.2 }
   then add --force-nft-hash-workaround=true here`)
	}

	hasNFTHashBug = bytes.Contains(output, []byte("16777216")) || bytes.Contains(output, []byte("0x01000000"))

	if hasNFTHashBug {
		klog.Info("nft vmap bug found, map indices will be affected by the workaround (0x01 will become 0x01000000)")
	}
}

func checkIPTableVersion() {
	cmdArr := [2]string{"ip6tables", "iptables"}
	for _, value := range cmdArr {
		cmd := exec.Command(value, "-V")
		stdout, err := cmd.Output()
		if err != nil && errors.Unwrap(err) != exec.ErrNotFound {
			klog.Warningf("cmd (%v) throws error: %v", cmd, err)
			continue
		}
		if bytes.Contains(stdout, []byte("legacy")) {
			klog.Warning("legacy ", value, " found")
		}
	}
}
