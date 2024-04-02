package common

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

// ReadFullPartnerNodeInfos reads partner node info and partner weight information from the specified paths and constructs
// a list of full bootstrap.NodeInfo for each partner node.
// Args:
// - log: logger used to log debug information.
// - partnerWeightsPath: path to partner weights configuration file.
// - partnerNodeInfoDir: path to partner nodes configuration file.
// Returns:
// - []bootstrap.NodeInfo: the generated node info list.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
func ReadFullPartnerNodeInfos(log zerolog.Logger, partnerWeightsPath, partnerNodeInfoDir string) ([]bootstrap.NodeInfo, error) {
	partners, err := ReadPartnerNodeInfos(partnerNodeInfoDir)
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("read %d partner node configuration files", len(partners))

	weights, err := ReadPartnerWeights(partnerWeightsPath)
	if err != nil {
		return nil, err
	}
	log.Info().Msgf("read %d weights for partner nodes", len(weights))

	var nodes []bootstrap.NodeInfo
	for _, partner := range partners {
		// validate every single partner node
		err = ValidateNodeID(partner.NodeID)
		if err != nil {
			return nil, fmt.Errorf("invalid node ID: %s", partner.NodeID)
		}
		err = ValidateNetworkPubKey(partner.NetworkPubKey)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("invalid network public key: %s", partner.NetworkPubKey))
		}
		err = ValidateStakingPubKey(partner.StakingPubKey)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("invalid staking public key: %s", partner.StakingPubKey))
		}
		weight := weights[partner.NodeID]
		if valid := ValidateWeight(weight); !valid {
			return nil, fmt.Errorf(fmt.Sprintf("invalid partner weight: %d", weight))
		}

		if weight != flow.DefaultInitialWeight {
			log.Warn().Msgf("partner node (id=%x) has non-default weight (%d != %d)", partner.NodeID, weight, flow.DefaultInitialWeight)
		}

		node := bootstrap.NewPublicNodeInfo(
			partner.NodeID,
			partner.Role,
			partner.Address,
			weight,
			partner.NetworkPubKey.PublicKey,
			partner.StakingPubKey.PublicKey,
		)
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// ReadPartnerWeights reads the partner weights configuration file and returns a list of PartnerWeights.
// Args:
// - partnerWeightsPath: path to partner weights configuration file.
// Returns:
// - PartnerWeights: the generated partner weights list.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
func ReadPartnerWeights(partnerWeightsPath string) (PartnerWeights, error) {
	var weights PartnerWeights

	err := ReadJSON(partnerWeightsPath, &weights)
	if err != nil {
		return nil, fmt.Errorf("failed to read partner weights json: %w", err)
	}
	return weights, nil
}

// ReadPartnerNodeInfos reads the partner node info from the configuration file and returns a list of []bootstrap.NodeInfoPub.
// Args:
// - partnerNodeInfoDir: path to partner nodes configuration file.
// Returns:
// - []bootstrap.NodeInfoPub: the generated partner node info list.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
func ReadPartnerNodeInfos(partnerNodeInfoDir string) ([]bootstrap.NodeInfoPub, error) {
	var partners []bootstrap.NodeInfoPub
	files, err := FilesInDir(partnerNodeInfoDir)
	if err != nil {
		return nil, fmt.Errorf("could not read partner node infos: %w", err)
	}
	for _, f := range files {
		// skip files that do not include node-infos
		if !strings.Contains(f, bootstrap.PathPartnerNodeInfoPrefix) {
			continue
		}
		// read file and append to partners
		var p bootstrap.NodeInfoPub
		err = ReadJSON(f, &p)
		if err != nil {
			return nil, fmt.Errorf("failed to read node info: %w", err)
		}
		partners = append(partners, p)
	}
	return partners, nil
}

// ReadFullInternalNodeInfos reads internal node info and internal node weight information from the specified paths and constructs
// a list of full bootstrap.NodeInfo for each internal node.
// Args:
// - log: logger used to log debug information.
// - internalNodePrivInfoDir: path to internal nodes  private info.
// - internalWeightsConfig: path to internal weights configuration file.
// Returns:
// - []bootstrap.NodeInfo: the generated node info list.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
func ReadFullInternalNodeInfos(log zerolog.Logger, internalNodePrivInfoDir, internalWeightsConfig string) ([]bootstrap.NodeInfo, error) {
	privInternals, err := ReadInternalNodeInfos(internalNodePrivInfoDir)
	if err != nil {
		return nil, err
	}
	
	log.Info().Msgf("read %v internal private node-info files", len(privInternals))

	weights := internalWeightsByAddress(log, internalWeightsConfig)
	log.Info().Msgf("read %d weights for internal nodes", len(weights))

	var nodes []bootstrap.NodeInfo
	for _, internal := range privInternals {
		// check if address is valid format
		ValidateAddressFormat(log, internal.Address)

		// validate every single internal node
		err := ValidateNodeID(internal.NodeID)
		if err != nil {
			return nil, fmt.Errorf(fmt.Sprintf("invalid internal node ID: %s", internal.NodeID))
		}
		weight := weights[internal.NodeID.String()]
		if valid := ValidateWeight(weight); !valid {
			return nil, fmt.Errorf(fmt.Sprintf("invalid partner weight: %d", weight))
		}
		if weight != flow.DefaultInitialWeight {
			log.Warn().Msgf("internal node (id=%x) has non-default weight (%d != %d)", internal.NodeID, weight, flow.DefaultInitialWeight)
		}

		node := bootstrap.NewPrivateNodeInfo(
			internal.NodeID,
			internal.Role,
			internal.Address,
			weight,
			internal.NetworkPrivKey,
			internal.StakingPrivKey,
		)

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// ReadInternalNodeInfos reads our internal node private infos generated by `keygen` command and returns it.
// Args:
// - internalNodePrivInfoDir: path to internal nodes  private info.
// Returns:
// - []bootstrap.NodeInfo: the generated private node info list.
// - error: if any error occurs. Any error returned from this function is irrecoverable.
func ReadInternalNodeInfos(internalNodePrivInfoDir string) ([]bootstrap.NodeInfoPriv, error) {
	var internalPrivInfos []bootstrap.NodeInfoPriv

	// get files in internal priv node infos directory
	files, err := FilesInDir(internalNodePrivInfoDir)
	if err != nil {
		return nil, fmt.Errorf("could not read partner node infos: %w", err)
	}

	// for each of the files
	for _, f := range files {
		// skip files that do not include node-infos
		if !strings.Contains(f, bootstrap.PathPrivNodeInfoPrefix) {
			continue
		}

		// read file and append to partners
		var p bootstrap.NodeInfoPriv
		err = ReadJSON(f, &p)
		if err != nil {
			return nil, fmt.Errorf("failed to read json: %w", err)
		}
		internalPrivInfos = append(internalPrivInfos, p)
	}

	return internalPrivInfos, nil
}

// internalWeightsByAddress returns a mapping of node address by weight for internal nodes
func internalWeightsByAddress(log zerolog.Logger, config string) map[string]uint64 {
	// read json
	var configs []bootstrap.NodeConfig
	err := ReadJSON(config, &configs)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read json")
	}
	log.Info().Interface("config", configs).Msgf("read internal node configurations")

	weights := make(map[string]uint64)
	for _, config := range configs {
		if _, ok := weights[config.Address]; !ok {
			weights[config.Address] = config.Weight
		} else {
			log.Error().Msgf("duplicate internal node address %s", config.Address)
		}
	}

	return weights
}
