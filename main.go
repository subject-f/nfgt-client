package main

import (
	"fmt"
	"time"

	"nfgt-client/pkg/client"
	"nfgt-client/pkg/common"
	"nfgt-client/pkg/git"

	"github.com/google/uuid"
)

func main() {
	c := common.ParseConfig("./config.yaml")
	g := git.NewGitProvider(c)

	start := time.Now()
	cli := client.NewClient(c, g)

	fmt.Printf("Sync took %v\n", time.Since(start))

	pendingTransaction, err := cli.Transact("qdTAeSFTWbOwmLiG", client.Transaction{
		TransactionId: uuid.NewString(),
		Asset: client.Asset{
			AssetId: common.EncodeToBase64("https://guya.moe/media/manga/Kaguya-Wants-To-Be-Confessed-To/chapters/0257_rk7zu4ws/7/01.png?v2"),
			OwnerId: common.EncodeToBase64("239583901788536832"),
			Metadata: map[string]interface{}{
				"some": "thing",
			},
		},
	})

	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(pendingTransaction.SuccessorPassphrase)
	}

	t, err := cli.Transact(pendingTransaction.SuccessorPassphrase, client.Transaction{
		TransactionId: uuid.NewString(),
		Asset: client.Asset{
			AssetId: common.EncodeToBase64("https://guya.moe/media/manga/Kaguya-Wants-To-Be-Confessed-To/chapters/0257_rk7zu4ws/7/01.png?v2"),
			OwnerId: common.EncodeToBase64("239583901788536832"),
			Metadata: map[string]interface{}{
				"some": "thing",
			},
		},
	})

	fmt.Println(t, err)

	select {}
}
