# amazongiftcard
amazon giftcard creat agcod service

## use 
```
	//fmt.Println(populatePayload("100"))
	err := amazon.DoCreateGiftCard("NA", 100, "USD", func(args ...any) {
		//遍历args
		for _, v := range args {
			fmt.Println(v)
		}
	})
	if err != nil {
		fmt.Println(err)
		return
	}
```
ps:参考https://github.com/leadstoloyals/node-agcod/tree/master
【https://github.com/leadstoloyals/node-agcod/tree/master】
