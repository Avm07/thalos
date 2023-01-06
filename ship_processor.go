package main

import (
	"encoding/hex"
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"eosio-ship-trace-reader/internal/redis"
	"github.com/eoscanada/eos-go/ship"
)

func encodeMessage(v interface{}) ([]byte, bool) {
	payload, err := json.Marshal(v)
	if err != nil {
		log.WithError(err).
			WithField("v", v).
			Warn("Failed to encode message to json")
		return nil, false
	}
	return payload, true
}

func queueMessage(key redis.Key, payload []byte) bool {
	err := redis.RegisterPublish(key.String(), payload).Err()
	if err != nil {
		log.WithError(err).Errorf("Failed to post to channel '%s'", key)
		return false
	}
	return true
}

func encodeQueue(channel redis.Channel, v interface{}) bool {
	if payload, ok := encodeMessage(v); ok {
		channel := redisNs.NewKey(channel)
		if queueMessage(channel, payload) {
			return true
		}
	}
	return false
}

func processBlock(block *ship.GetBlocksResultV0) {
	if block.ThisBlock.BlockNum%100 == 0 {
		log.Infof("Current: %d, Head: %d\n", block.ThisBlock.BlockNum, block.Head.BlockNum)
	}

	if block.ThisBlock.BlockNum%10 == 0 {
		hb := HearthBeat{
			BlockNum:                 block.ThisBlock.BlockNum,
			LastIrreversibleBlockNum: block.LastIrreversible.BlockNum,
			HeadBlockNum:             block.Head.BlockNum,
		}

		encodeQueue(redis.HeartbeatChannel, hb)

		_, err := redis.Send()
		if err != nil {
			log.WithError(err).Error("Failed to send redis")
		}
	}
}

func processTraces(traces []*ship.TransactionTraceV0) {
	for _, trace := range traces {

		encodeQueue(redis.TransactionChannel, trace)

		// Actions
		for _, actionTraceVar := range trace.ActionTraces {
			act_trace := actionTraceVar.Impl.(*ship.ActionTraceV0)

			act := ActionTrace{
				TxID:     trace.ID,
				Receiver: act_trace.Receiver,
				Contract: act_trace.Act.Account,
				Action:   act_trace.Act.Name,
				HexData:  hex.EncodeToString(act_trace.Act.Data),
			}

			abi, err := GetAbi(act_trace.Act.Account)
			if err == nil {
				v, err := DecodeAction(abi, act_trace.Act.Data, act_trace.Act.Name)
				if err != nil {
					log.WithError(err).Warn("Failed to decode action")
				}
				act.Data = v
			} else {
				log.WithError(err).Errorf("Failed to get abi for contract %s", act_trace.Act.Account)
			}

			payload, ok := encodeMessage(act)
			if !ok {
				continue
			}

			channels := []redis.ChannelInterface{
				redis.ActionChannel{},
				redis.ActionChannel{Action: string(act.Action)},
				redis.ActionChannel{Contract: string(act.Contract)},
				redis.ActionChannel{Action: string(act.Action), Contract: string(act.Contract)},
			}

			for _, channel := range channels {
				queueMessage(redisNs.NewKey(channel), payload)
			}
		}
	}

	_, err := redis.Send()
	if err != nil {
		log.WithError(err).Error("Failed to send redis")
	}
}
