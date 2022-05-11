package convert

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// This file contains service event fixtures for testing purposes.
// The Cadence form is represented by JSON-CDC-encoded string variables.

// EpochSetupFixture returns an EpochSetup service event as a Cadence event
// representation and as a protocol model representation.
func EpochSetupFixture(chain flow.ChainID) (flow.Event, *flow.EpochSetup) {
	events, err := systemcontracts.ServiceEventsForChain(chain)
	if err != nil {
		panic(err)
	}

	event := unittest.EventFixture(events.EpochSetup.EventType(), 1, 1, unittest.IdentifierFixture(), 0)
	event.Payload = []byte(epochSetupFixtureJSON)

	// randomSource is [0,0,...,1,2,3,4]
	randomSource := make([]uint8, flow.EpochSetupRandomSourceLength)
	for i := 0; i < 4; i++ {
		randomSource[flow.EpochSetupRandomSourceLength-1-i] = uint8(4 - i)
	}

	expected := &flow.EpochSetup{
		Counter:            1,
		FirstView:          100,
		FinalView:          200,
		DKGPhase1FinalView: 150,
		DKGPhase2FinalView: 160,
		DKGPhase3FinalView: 170,
		RandomSource:       randomSource,
		Assignments: flow.AssignmentList{
			{
				flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001"),
				flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002"),
			},
			{
				flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000003"),
				flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000004"),
			},
		},
		Participants: flow.IdentityList{
			{
				Role:          flow.RoleCollection,
				NodeID:        flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001"),
				Address:       "1.flow.com",
				NetworkPubKey: unittest.MustDecodePublicKeyHex(crypto.ECDSAP256, "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"),
				StakingPubKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"),
				Weight:        100,
			},
			{
				Role:          flow.RoleCollection,
				NodeID:        flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002"),
				Address:       "2.flow.com",
				NetworkPubKey: unittest.MustDecodePublicKeyHex(crypto.ECDSAP256, "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"),
				StakingPubKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"),
				Weight:        100,
			},
			{
				Role:          flow.RoleCollection,
				NodeID:        flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000003"),
				Address:       "3.flow.com",
				NetworkPubKey: unittest.MustDecodePublicKeyHex(crypto.ECDSAP256, "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"),
				StakingPubKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"),
				Weight:        100,
			},
			{
				Role:          flow.RoleCollection,
				NodeID:        flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000004"),
				Address:       "4.flow.com",
				NetworkPubKey: unittest.MustDecodePublicKeyHex(crypto.ECDSAP256, "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"),
				StakingPubKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"),
				Weight:        100,
			},
			{
				Role:          flow.RoleConsensus,
				NodeID:        flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000011"),
				Address:       "11.flow.com",
				NetworkPubKey: unittest.MustDecodePublicKeyHex(crypto.ECDSAP256, "cfdfe8e4362c8f79d11772cb7277ab16e5033a63e8dd5d34caf1b041b77e5b2d63c2072260949ccf8907486e4cfc733c8c42ca0e4e208f30470b0d950856cd47"),
				StakingPubKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "8207559cd7136af378bba53a8f0196dee3849a3ab02897c1995c3e3f6ca0c4a776c3ae869d1ddbb473090054be2400ad06d7910aa2c5d1780220fdf3765a3c1764bce10c6fe66a5a2be51a422e878518bd750424bb56b8a0ecf0f8ad2057e83f"),
				Weight:        100,
			},
			{
				Role:          flow.RoleExecution,
				NodeID:        flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000021"),
				Address:       "21.flow.com",
				NetworkPubKey: unittest.MustDecodePublicKeyHex(crypto.ECDSAP256, "d64318ba0dbf68f3788fc81c41d507c5822bf53154530673127c66f50fe4469ccf1a054a868a9f88506a8999f2386d86fcd2b901779718cba4fb53c2da258f9e"),
				StakingPubKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "880b162b7ec138b36af401d07868cb08d25746d905395edbb4625bdf105d4bb2b2f4b0f4ae273a296a6efefa7ce9ccb914e39947ce0e83745125cab05d62516076ff0173ed472d3791ccef937597c9ea12381d76f547a092a4981d77ff3fba83"),
				Weight:        100,
			},
			{
				Role:          flow.RoleVerification,
				NodeID:        flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000031"),
				Address:       "31.flow.com",
				NetworkPubKey: unittest.MustDecodePublicKeyHex(crypto.ECDSAP256, "697241208dcc9142b6f53064adc8ff1c95760c68beb2ba083c1d005d40181fd7a1b113274e0163c053a3addd47cd528ec6a1f190cf465aac87c415feaae011ae"),
				StakingPubKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "b1f97d0a06020eca97352e1adde72270ee713c7daf58da7e74bf72235321048b4841bdfc28227964bf18e371e266e32107d238358848bcc5d0977a0db4bda0b4c33d3874ff991e595e0f537c7b87b4ddce92038ebc7b295c9ea20a1492302aa7"),
				Weight:        100,
			},
		},
	}

	return event, expected
}

// EpochCommitFixture returns an EpochCommit service event as a Cadence event
// representation and as a protocol model representation.
func EpochCommitFixture(chain flow.ChainID) (flow.Event, *flow.EpochCommit) {

	events, err := systemcontracts.ServiceEventsForChain(chain)
	if err != nil {
		panic(err)
	}

	event := unittest.EventFixture(events.EpochCommit.EventType(), 1, 1, unittest.IdentifierFixture(), 0)
	event.Payload = []byte(epochCommitFixtureJSON)

	expected := &flow.EpochCommit{
		Counter: 1,
		ClusterQCs: []flow.ClusterQCVoteData{
			{
				VoterIDs: []flow.Identifier{
					flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001"),
					flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002"),
				},
				SigData: unittest.MustDecodeSignatureHex("b072ed22ed305acd44818a6c836e09b4e844eebde6a4fdbf5cec983e2872b86c8b0f6c34c0777bf52e385ab7c45dc55d"),
			},
			{
				VoterIDs: []flow.Identifier{
					flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000003"),
					flow.MustHexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000004"),
				},
				SigData: unittest.MustDecodeSignatureHex("899e266a543e1b3a564f68b22f7be571f2e944ec30fadc4b39e2d5f526ba044c0f3cb2648f8334fc216fa3360a0418b2"),
			},
		},
		DKGGroupKey: unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "8c588266db5f5cda629e83f8aa04ae9413593fac19e4865d06d291c9d14fbdd9bdb86a7a12f9ef8590c79cb635e3163315d193087e9336092987150d0cd2b14ac6365f7dc93eec573752108b8c12368abb65f0652d9f644e5aed611c37926950"),
		DKGParticipantKeys: []crypto.PublicKey{
			unittest.MustDecodePublicKeyHex(crypto.BLSBLS12381, "87a339e4e5c74f089da20a33f515d8c8f4464ab53ede5a74aa2432cd1ae66d522da0c122249ee176cd747ddc83ca81090498389384201614caf51eac392c1c0a916dfdcfbbdf7363f9552b6468434add3d3f6dc91a92bbe3ee368b59b7828488"),
		},
	}

	return event, expected
}

var epochSetupFixtureJSON = `
{
  "type": "Event",
  "value": {
    "id": "A.01cf0e2f2f715450.FlowEpoch.EpochSetup",
    "fields": [
      {
        "name": "counter",
        "value": {
          "type": "UInt64",
          "value": "1"
        }
      },
      {
        "name": "nodeInfo",
        "value": {
          "type": "Array",
          "value": [
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo",
                "fields": [
                  {
                    "name": "id",
                    "value": {
                      "type": "String",
                      "value": "0000000000000000000000000000000000000000000000000000000000000001"
                    }
                  },
                  {
                    "name": "role",
                    "value": {
                      "type": "UInt8",
                      "value": "1"
                    }
                  },
                  {
                    "name": "networkingAddress",
                    "value": {
                      "type": "String",
                      "value": "1.flow.com"
                    }
                  },
                  {
                    "name": "networkingKey",
                    "value": {
                      "type": "String",
                      "value": "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"
                    }
                  },
                  {
                    "name": "stakingKey",
                    "value": {
                      "type": "String",
                      "value": "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"
                    }
                  },
                  {
                    "name": "tokensStaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensCommitted",
                    "value": {
                      "type": "UFix64",
                      "value": "1350000.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaking",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensRewarded",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "delegators",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  },
                  {
                    "name": "delegatorIDCounter",
                    "value": {
                      "type": "UInt32",
                      "value": "0"
                    }
                  },
                  {
                    "name": "tokensRequestedToUnstake",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "initialWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  }
                ]
              }
            },
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo",
                "fields": [
                  {
                    "name": "id",
                    "value": {
                      "type": "String",
                      "value": "0000000000000000000000000000000000000000000000000000000000000002"
                    }
                  },
                  {
                    "name": "role",
                    "value": {
                      "type": "UInt8",
                      "value": "1"
                    }
                  },
                  {
                    "name": "networkingAddress",
                    "value": {
                      "type": "String",
                      "value": "2.flow.com"
                    }
                  },
                  {
                    "name": "networkingKey",
                    "value": {
                      "type": "String",
                      "value": "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"
                    }
                  },
                  {
                    "name": "stakingKey",
                    "value": {
                      "type": "String",
                      "value": "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"
                    }
                  },
                  {
                    "name": "tokensStaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensCommitted",
                    "value": {
                      "type": "UFix64",
                      "value": "1350000.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaking",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensRewarded",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "delegators",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  },
                  {
                    "name": "delegatorIDCounter",
                    "value": {
                      "type": "UInt32",
                      "value": "0"
                    }
                  },
                  {
                    "name": "tokensRequestedToUnstake",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "initialWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  }
                ]
              }
            },
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo",
                "fields": [
                  {
                    "name": "id",
                    "value": {
                      "type": "String",
                      "value": "0000000000000000000000000000000000000000000000000000000000000003"
                    }
                  },
                  {
                    "name": "role",
                    "value": {
                      "type": "UInt8",
                      "value": "1"
                    }
                  },
                  {
                    "name": "networkingAddress",
                    "value": {
                      "type": "String",
                      "value": "3.flow.com"
                    }
                  },
                  {
                    "name": "networkingKey",
                    "value": {
                      "type": "String",
                      "value": "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"
                    }
                  },
                  {
                    "name": "stakingKey",
                    "value": {
                      "type": "String",
                      "value": "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"
                    }
                  },
                  {
                    "name": "tokensStaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensCommitted",
                    "value": {
                      "type": "UFix64",
                      "value": "1350000.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaking",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensRewarded",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "delegators",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  },
                  {
                    "name": "delegatorIDCounter",
                    "value": {
                      "type": "UInt32",
                      "value": "0"
                    }
                  },
                  {
                    "name": "tokensRequestedToUnstake",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "initialWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  }
                ]
              }
            },
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo",
                "fields": [
                  {
                    "name": "id",
                    "value": {
                      "type": "String",
                      "value": "0000000000000000000000000000000000000000000000000000000000000004"
                    }
                  },
                  {
                    "name": "role",
                    "value": {
                      "type": "UInt8",
                      "value": "1"
                    }
                  },
                  {
                    "name": "networkingAddress",
                    "value": {
                      "type": "String",
                      "value": "4.flow.com"
                    }
                  },
                  {
                    "name": "networkingKey",
                    "value": {
                      "type": "String",
                      "value": "378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"
                    }
                  },
                  {
                    "name": "stakingKey",
                    "value": {
                      "type": "String",
                      "value": "af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"
                    }
                  },
                  {
                    "name": "tokensStaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensCommitted",
                    "value": {
                      "type": "UFix64",
                      "value": "1350000.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaking",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensRewarded",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "delegators",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  },
                  {
                    "name": "delegatorIDCounter",
                    "value": {
                      "type": "UInt32",
                      "value": "0"
                    }
                  },
                  {
                    "name": "tokensRequestedToUnstake",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "initialWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  }
                ]
              }
            },
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo",
                "fields": [
                  {
                    "name": "id",
                    "value": {
                      "type": "String",
                      "value": "0000000000000000000000000000000000000000000000000000000000000011"
                    }
                  },
                  {
                    "name": "role",
                    "value": {
                      "type": "UInt8",
                      "value": "2"
                    }
                  },
                  {
                    "name": "networkingAddress",
                    "value": {
                      "type": "String",
                      "value": "11.flow.com"
                    }
                  },
                  {
                    "name": "networkingKey",
                    "value": {
                      "type": "String",
                      "value": "cfdfe8e4362c8f79d11772cb7277ab16e5033a63e8dd5d34caf1b041b77e5b2d63c2072260949ccf8907486e4cfc733c8c42ca0e4e208f30470b0d950856cd47"
                    }
                  },
                  {
                    "name": "stakingKey",
                    "value": {
                      "type": "String",
                      "value": "8207559cd7136af378bba53a8f0196dee3849a3ab02897c1995c3e3f6ca0c4a776c3ae869d1ddbb473090054be2400ad06d7910aa2c5d1780220fdf3765a3c1764bce10c6fe66a5a2be51a422e878518bd750424bb56b8a0ecf0f8ad2057e83f"
                    }
                  },
                  {
                    "name": "tokensStaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensCommitted",
                    "value": {
                      "type": "UFix64",
                      "value": "1350000.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaking",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensRewarded",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "delegators",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  },
                  {
                    "name": "delegatorIDCounter",
                    "value": {
                      "type": "UInt32",
                      "value": "0"
                    }
                  },
                  {
                    "name": "tokensRequestedToUnstake",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "initialWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  }
                ]
              }
            },
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo",
                "fields": [
                  {
                    "name": "id",
                    "value": {
                      "type": "String",
                      "value": "0000000000000000000000000000000000000000000000000000000000000021"
                    }
                  },
                  {
                    "name": "role",
                    "value": {
                      "type": "UInt8",
                      "value": "3"
                    }
                  },
                  {
                    "name": "networkingAddress",
                    "value": {
                      "type": "String",
                      "value": "21.flow.com"
                    }
                  },
                  {
                    "name": "networkingKey",
                    "value": {
                      "type": "String",
                      "value": "d64318ba0dbf68f3788fc81c41d507c5822bf53154530673127c66f50fe4469ccf1a054a868a9f88506a8999f2386d86fcd2b901779718cba4fb53c2da258f9e"
                    }
                  },
                  {
                    "name": "stakingKey",
                    "value": {
                      "type": "String",
                      "value": "880b162b7ec138b36af401d07868cb08d25746d905395edbb4625bdf105d4bb2b2f4b0f4ae273a296a6efefa7ce9ccb914e39947ce0e83745125cab05d62516076ff0173ed472d3791ccef937597c9ea12381d76f547a092a4981d77ff3fba83"
                    }
                  },
                  {
                    "name": "tokensStaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensCommitted",
                    "value": {
                      "type": "UFix64",
                      "value": "1350000.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaking",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensRewarded",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "delegators",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  },
                  {
                    "name": "delegatorIDCounter",
                    "value": {
                      "type": "UInt32",
                      "value": "0"
                    }
                  },
                  {
                    "name": "tokensRequestedToUnstake",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "initialWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  }
                ]
              }
            },
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo",
                "fields": [
                  {
                    "name": "id",
                    "value": {
                      "type": "String",
                      "value": "0000000000000000000000000000000000000000000000000000000000000031"
                    }
                  },
                  {
                    "name": "role",
                    "value": {
                      "type": "UInt8",
                      "value": "4"
                    }
                  },
                  {
                    "name": "networkingAddress",
                    "value": {
                      "type": "String",
                      "value": "31.flow.com"
                    }
                  },
                  {
                    "name": "networkingKey",
                    "value": {
                      "type": "String",
                      "value": "697241208dcc9142b6f53064adc8ff1c95760c68beb2ba083c1d005d40181fd7a1b113274e0163c053a3addd47cd528ec6a1f190cf465aac87c415feaae011ae"
                    }
                  },
                  {
                    "name": "stakingKey",
                    "value": {
                      "type": "String",
                      "value": "b1f97d0a06020eca97352e1adde72270ee713c7daf58da7e74bf72235321048b4841bdfc28227964bf18e371e266e32107d238358848bcc5d0977a0db4bda0b4c33d3874ff991e595e0f537c7b87b4ddce92038ebc7b295c9ea20a1492302aa7"
                    }
                  },
                  {
                    "name": "tokensStaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensCommitted",
                    "value": {
                      "type": "UFix64",
                      "value": "1350000.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaking",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensUnstaked",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "tokensRewarded",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "delegators",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  },
                  {
                    "name": "delegatorIDCounter",
                    "value": {
                      "type": "UInt32",
                      "value": "0"
                    }
                  },
                  {
                    "name": "tokensRequestedToUnstake",
                    "value": {
                      "type": "UFix64",
                      "value": "0.00000000"
                    }
                  },
                  {
                    "name": "initialWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      {
        "name": "firstView",
        "value": {
          "type": "UInt64",
          "value": "100"
        }
      },
      {
        "name": "finalView",
        "value": {
          "type": "UInt64",
          "value": "200"
        }
      },
      {
        "name": "collectorClusters",
        "value": {
          "type": "Array",
          "value": [
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowClusterQC.Cluster",
                "fields": [
                  {
                    "name": "index",
                    "value": {
                      "type": "UInt16",
                      "value": "0"
                    }
                  },
                  {
                    "name": "nodeWeights",
                    "value": {
                      "type": "Dictionary",
                      "value": [
                        {
                          "key": {
                            "type": "String",
                            "value": "0000000000000000000000000000000000000000000000000000000000000001"
                          },
                          "value": {
                            "type": "UInt64",
                            "value": "100"
                          }
                        },
						{
                          "key": {
                            "type": "String",
                            "value": "0000000000000000000000000000000000000000000000000000000000000002"
                          },
                          "value": {
                            "type": "UInt64",
                            "value": "100"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "name": "totalWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "100"
                    }
                  },
                  {
                    "name": "votes",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  }
                ]
              }
            },
            {
              "type": "Struct",
              "value": {
                "id": "A.01cf0e2f2f715450.FlowClusterQC.Cluster",
                "fields": [
                  {
                    "name": "index",
                    "value": {
                      "type": "UInt16",
                      "value": "1"
                    }
                  },
                  {
                    "name": "nodeWeights",
                    "value": {
                      "type": "Dictionary",
                      "value": [
                        {
                          "key": {
                            "type": "String",
                            "value": "0000000000000000000000000000000000000000000000000000000000000003"
                          },
                          "value": {
                            "type": "UInt64",
                            "value": "100"
                          }
                        },
						{
                          "key": {
                            "type": "String",
                            "value": "0000000000000000000000000000000000000000000000000000000000000004"
                          },
                          "value": {
                            "type": "UInt64",
                            "value": "100"
                          }
                        }
                      ]
                    }
                  },
                  {
                    "name": "totalWeight",
                    "value": {
                      "type": "UInt64",
                      "value": "0"
                    }
                  },
                  {
                    "name": "votes",
                    "value": {
                      "type": "Array",
                      "value": []
                    }
                  }
                ]
              }
            }
          ]
        }
      },
      {
        "name": "randomSource",
        "value": {
          "type": "String",
          "value": "01020304"
        }
      },
      {
        "name": "DKGPhase1FinalView",
        "value": {
          "type": "UInt64",
          "value": "150"
        }
      },
      {
        "name": "DKGPhase2FinalView",
        "value": {
          "type": "UInt64",
          "value": "160"
        }
      },
      {
        "name": "DKGPhase3FinalView",
        "value": {
          "type": "UInt64",
          "value": "170"
        }
      }
    ]
  }
}
`

var epochCommitFixtureJSON = `
{
    "type": "Event",
    "value": {
        "id": "A.01cf0e2f2f715450.FlowEpoch.EpochCommitted",
        "fields": [
            {
                "name": "counter",
                "value": {
                    "type": "UInt64",
                    "value": "1"
                }
            },
            {
                "name": "clusterQCs",
                "value": {
                    "type": "Array",
                    "value": [
                        {
                            "type": "Struct",
                            "value": {
                                "id": "A.01cf0e2f2f715450.FlowClusterQC.ClusterQC",
                                "fields": [
                                    {
                                        "name": "index",
                                        "value": {
                                            "type": "UInt16",
                                            "value": "0"
                                        }
                                    },
                                    {
                                        "name": "voteSignatures",
                                        "value": {
                                            "type": "Array",
                                            "value": [
                                                {
                                                    "type": "String",
                                                    "value": "a39cd1e1bf7e2fb0609b7388ce5215a6a4c01eef2aee86e1a007faa28a6b2a3dc876e11bb97cdb26c3846231d2d01e4d"
                                                },
                                                {
                                                    "type": "String",
                                                    "value": "91673ad9c717d396c9a0953617733c128049ac1a639653d4002ab245b121df1939430e313bcbfd06948f6a281f6bf853"
                                                }
                                            ]
                                        }
                                    },
									{
										"name": "voteMessage",
										"value": {
											"type": "String",
											"value": "irrelevant_for_these_purposes"
										}
									},
									{
                                        "name": "voterIDs",
                                        "value": {
                                            "type": "Array",
                                            "value": [
                                                {
                                                    "type": "String",
                                                    "value": "0000000000000000000000000000000000000000000000000000000000000001"
                                                },
                                                {
                                                    "type": "String",
                                                    "value": "0000000000000000000000000000000000000000000000000000000000000002"
                                                }
                                            ]
                                        }
                                    }
                                ]
                            }
                        },
                        {
                            "type": "Struct",
                            "value": {
                                "id": "A.01cf0e2f2f715450.FlowClusterQC.ClusterQC",
                                "fields": [
                                    {
                                        "name": "index",
                                        "value": {
                                            "type": "UInt16",
                                            "value": "1"
                                        }
                                    },
                                    {
                                        "name": "voteSignatures",
                                        "value": {
                                            "type": "Array",
                                            "value": [
                                                {
                                                    "type": "String",
                                                    "value": "b2bff159971852ed63e72c37991e62c94822e52d4fdcd7bf29aaf9fb178b1c5b4ce20dd9594e029f3574cb29533b857a"
                                                },
                                                {
                                                    "type": "String",
                                                    "value": "9931562f0248c9195758da3de4fb92f24fa734cbc20c0cb80280163560e0e0348f843ac89ecbd3732e335940c1e8dccb"
                                                }
                                            ]
                                        }
                                    },
									{
										"name": "voteMessage",
										"value": {
											"type": "String",
											"value": "irrelevant_for_these_purposes"
										}
									},
									{
                                        "name": "voterIDs",
                                        "value": {
                                            "type": "Array",
                                            "value": [
                                                {
                                                    "type": "String",
                                                    "value": "0000000000000000000000000000000000000000000000000000000000000003"
                                                },
                                                {
                                                    "type": "String",
                                                    "value": "0000000000000000000000000000000000000000000000000000000000000004"
                                                }
                                            ]
                                        }
                                    }
                                ]
                            }
                        }
                    ]
                }
            },
            {
                "name": "dkgPubKeys",
                "value": {
                    "type": "Array",
                    "value": [
                        {
                            "type": "String",
                            "value": "8c588266db5f5cda629e83f8aa04ae9413593fac19e4865d06d291c9d14fbdd9bdb86a7a12f9ef8590c79cb635e3163315d193087e9336092987150d0cd2b14ac6365f7dc93eec573752108b8c12368abb65f0652d9f644e5aed611c37926950"
                        },
                        {
                            "type": "String",
                            "value": "87a339e4e5c74f089da20a33f515d8c8f4464ab53ede5a74aa2432cd1ae66d522da0c122249ee176cd747ddc83ca81090498389384201614caf51eac392c1c0a916dfdcfbbdf7363f9552b6468434add3d3f6dc91a92bbe3ee368b59b7828488"
                        }
                    ]
                }
            }
        ]
    }
}`
