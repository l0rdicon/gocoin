package main

import (
	"os"
	"fmt"
	"time"
	"github.com/piotrnar/gocoin/btc"
	"github.com/piotrnar/gocoin/blockdb"
)

var (
	GocoinHomeDir string
	StartTime time.Time
)


func BitcoinHome() (res string) {
	res = os.Getenv("APPDATA")
	if res!="" {
		res += "\\Bitcoin\\"
		return
	}
	res = os.Getenv("HOME")
	if res!="" {
		res += "/.bitcoin/"
	}
	return
}


func host_init() {
	BtcRootDir := BitcoinHome()

	if *datadir == "" {
		GocoinHomeDir = BtcRootDir+"gocoin/"
	} else {
		GocoinHomeDir = *datadir+"/"
	}

	if *testnet { // testnet3
		DefaultTcpPort = 18333
		GenesisBlock = btc.NewUint256FromString("000000000933ea01ad0ee984209779baaec3ced90fa3f408719526f8d77f4943")
		Magic = [4]byte{0x0B,0x11,0x09,0x07}
		GocoinHomeDir += "tstnet/"
		AddrVersion = 0x6f
		BtcRootDir += "testnet3/"
	} else {
		DefaultTcpPort = 8333
		GenesisBlock = btc.NewUint256FromString("000000000019d6689c085ae165831e934ff763ae46a2a6c172b3f1b60a8ce26f")
		Magic = [4]byte{0xF9,0xBE,0xB4,0xD9}
		GocoinHomeDir += "btcnet/"
		AddrVersion = 0x00
	}

	fi, e := os.Stat(GocoinHomeDir+"blockchain.idx")
	if e!=nil || fi.Size()<100 {
		os.RemoveAll(GocoinHomeDir)
		fmt.Println("You seem to be running Gocoin for the fist time on this PC")
		fi, e = os.Stat(BtcRootDir+"blocks/blk00000.dat")
		if e==nil && fi.Size()>1024*1024 {
			fmt.Println("There is a database from Satoshi client on your disk...")
			if ask_yes_no("Go you want to import this database into Gocoin?") {
				import_blockchain(BtcRootDir+"blocks")
			}
		}
	}

	fmt.Println("Opening blockchain...")
	sta := time.Now().UnixNano()
	BlockChain = btc.NewChain(GocoinHomeDir, GenesisBlock, *rescan)
	sto := time.Now().UnixNano()
	fmt.Printf("Blockchain open in %.3f seconds\n", float64(sto-sta)/1e9)
	if *nosync {
		BlockChain.DoNotSync = true
		fmt.Println("Syncing is disabled. Switch it on with 'sync' command")
	}
	BlockChain.Unspent.SetTxNotify(TxNotify)
	StartTime = time.Now()
}


func stat(totnsec, pernsec int64, totbytes, perbytes uint64, height uint32) {
	totmbs := float64(totbytes) / (1024*1024)
	perkbs := float64(perbytes) / (1024)
	var x string
	if btc.EcdsaVerifyCnt > 0 {
		x = fmt.Sprintf("|  %d -> %d us/ecdsa", btc.EcdsaVerifyCnt, uint64(pernsec)/btc.EcdsaVerifyCnt/1e3)
		btc.EcdsaVerifyCnt = 0
	}
	fmt.Printf("%.1fMB of data processed. We are at height %d. Processing speed %.3fMB/sec, recent: %.1fKB/s %s\n",
		totmbs, height, totmbs/(float64(totnsec)/1e9), perkbs/(float64(pernsec)/1e9), x)
}


func import_blockchain(dir string) {
	trust := !ask_yes_no("Go you want to verify scripts while importing (will be slow)?")

	BlockDatabase := blockdb.NewBlockDB(dir, Magic)
	chain := btc.NewChain(GocoinHomeDir, GenesisBlock, false)

	var bl *btc.Block
	var er error
	var dat []byte
	var totbytes, perbytes uint64

	chain.DoNotSync = true

	fmt.Println("Be patient while importing Satoshi's database... ")
	start := time.Now().UnixNano()
	prv := start
	for {
		now := time.Now().UnixNano()
		if now-prv >= 10e9 {
			stat(now-start, now-prv, totbytes, perbytes, chain.BlockTreeEnd.Height)
			prv = now  // show progress each 10 seconds
			perbytes = 0
		}

		dat, er = BlockDatabase.FetchNextBlock()
		if dat==nil || er!=nil {
			println("END of DB file")
			break
		}

		bl, er = btc.NewBlock(dat[:])
		if er != nil {
			println("Block inconsistent:", er.Error())
			break
		}

		bl.Trusted = trust

		er, _, _ = chain.CheckBlock(bl)

		if er != nil {
			if er.Error()!="Genesis" {
				println("CheckBlock failed:", er.Error())
				os.Exit(1) // Such a thing should not happen, so let's better abort here.
			}
			continue
		}

		er = chain.AcceptBlock(bl)
		if er != nil {
			println("AcceptBlock failed:", er.Error())
			os.Exit(1) // Such a thing should not happen, so let's better abort here.
		}

		totbytes += uint64(len(bl.Raw))
		perbytes += uint64(len(bl.Raw))
	}

	stop := time.Now().UnixNano()
	stat(stop-start, stop-prv, totbytes, perbytes, chain.BlockTreeEnd.Height)

	fmt.Println("Satoshi's database import finished in", (stop-start)/1e9, "seconds")

	fmt.Println("Now saving the new database...")
	chain.Save()
	chain.Close()
	fmt.Println("Database saved. No more imports should be needed.")
}
