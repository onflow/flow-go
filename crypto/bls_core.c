// +build relic

#include "bls_include.h"

// this file is about the core functions required by the BLS signature scheme

// The functions are tested for ALLOC=AUTO (not for ALLOC=DYNAMIC)

// functions to export macros to the Go layer (because cgo does not import macros)
int get_signature_len() {
    return SIGNATURE_LEN;
}

int get_pk_len() {
    return PK_LEN;
}

int get_sk_len() {
    return SK_LEN;
}

// checks an input scalar a satisfies 0 < a < r
// where (r) is the order of G1/G2
int check_membership_Zr(const bn_t a){
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    if (bn_cmp(a,r) != RLC_LT || bn_cmp_dig(a, 0) != RLC_GT) 
        return INVALID; 
    return VALID;
}

// checks if input point s is on the curve E1
// and is in the subgroup G1. 
int check_membership_G1(const ep_t p){
#if MEMBERSHIP_CHECK
    // check p is on curve
    if (!ep_on_curve(p))
        return INVALID;
    // check p is in G1
    #if MEMBERSHIP_CHECK_G1 == EXP_ORDER
    return simple_subgroup_check_G1(p);
    #elif MEMBERSHIP_CHECK_G1 == BOWE
    // section 3.2 from https://eprint.iacr.org/2019/814.pdf
    return bowe_subgroup_check_G1(p);
    #else
    return UNDEFINED;
    #endif
#endif
    return VALID;
}

// checks if input point s is on the curve E2 
// and is in the subgroup G2.
// 
// membership check in G2 is using a scalar multiplication by the group order.
// TODO: switch to the faster Bowe check 
int check_membership_G2(const ep2_t p){
#if MEMBERSHIP_CHECK
    // check p is on curve
    if (!ep2_on_curve((ep2_st*)p))
        return INVALID;
    // check p is in G2
    #if MEMBERSHIP_CHECK_G2 == EXP_ORDER
    return simple_subgroup_check_G2(p);
    #elif MEMBERSHIP_CHECK_G2 == BOWE
    // TODO: implement Bowe's check
    return UNDEFINED;
    #else
    return UNDEFINED;
    #endif
#endif
    return VALID;
}

// Computes a BLS signature from a G1 point 
static void bls_sign_ep(byte* s, const bn_t sk, const ep_t h) {
    ep_t p;
    ep_new(p);
    // s = h^sk
    ep_mult(p, h, sk);
    ep_write_bin_compact(s, p, SIGNATURE_LEN);
    ep_free(p);
}

// Computes a BLS signature from a hash
void bls_sign(byte* s, const bn_t sk, const byte* data, const int len) {
    ep_t h;
    ep_new(h);
    // hash to G1
    map_to_G1(h, data, len);
    // s = h^sk
    bls_sign_ep(s, sk, h);
    ep_free(h);
}

// Verifies a BLS signature (G1 point) against a public key (G2 point)
// and a message data.
// The signature and public key are assumed to be in G1 and G2 respectively. This 
// function only checks the pairing equality. 
static int bls_verify_ep(const ep2_t pk, const ep_t s, const byte* data, const int len) { 
    
    ep_t elemsG1[2];
    ep2_t elemsG2[2];

    // elemsG1[0] = s
    ep_new(elemsG1[0]);
    ep_copy(elemsG1[0], (ep_st*)s);

    // elemsG1[1] = h
    ep_new(elemsG1[1]);
    // hash to G1 
    map_to_G1(elemsG1[1], data, len); 

    // elemsG2[1] = pk
    ep2_new(elemsG2[1]);
    ep2_copy(elemsG2[1], (ep2_st*)pk);

#if DOUBLE_PAIRING  
    // elemsG2[0] = -g2
    ep2_new(&elemsG2[0]);
    ep2_neg(elemsG2[0], core_get()->ep2_g); // could be hardcoded 

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), 2);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

#elif SINGLE_PAIRING   
    fp12_t pair1, pair2;
    fp12_new(&pair1); fp12_new(&pair2);
    pp_map_oatep_k12(pair1, elemsG1[0], core_get()->ep2_g);
    pp_map_oatep_k12(pair2, elemsG1[1], elemsG2[1]);

    int res = fp12_cmp(pair1, pair2);
#endif
    ep_free(elemsG1[0]);
    ep_free(elemsG1[1]);
    ep2_free(elemsG2[0]);
    ep2_free(elemsG2[1]);
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) {
        return VALID;
    }
    return INVALID;
}


// Verifies the validity of an aggregated BLS signature under distinct messages.
//
// Each message is mapped to a set of public keys, so that the verification equation is 
// optimized to compute one pairing per message. 
// - sig is the signature.
// - nb_hashes is the number of the messages (hashes) in the map
// - hashes is pointer to all flattened hashes in order where the hash at index i has a byte length len_hashes[i],
//   is mapped to pks_per_hash[i] public keys. 
// - the keys are flattened in pks in the same hashes order.
//
// membership check of the signature in G1 is verified in this function
// membership check of pks in G2 is not verified in this function
// the membership check is separated to allow optimizing multiple verifications using the same pks
int bls_verifyPerDistinctMessage(const byte* sig, 
                         const int nb_hashes, const byte* hashes, const uint32_t* len_hashes,
                         const uint32_t* pks_per_hash, const ep2_st* pks) {  
    
    ep_t* elemsG1 = (ep_t*)malloc((nb_hashes + 1) * sizeof(ep_t));
    ep2_t* elemsG2 = (ep2_t*)malloc((nb_hashes + 1) * sizeof(ep2_t));

    // elemsG1[0] = sig
    ep_new(elemsG1[0]);
    int read_ret = ep_read_bin_compact(elemsG1[0], sig, SIGNATURE_LEN);
    if (read_ret != RLC_OK) 
        return read_ret;

    // check s is on curve and in G1
    if (check_membership_G1(elemsG1[0]) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID;

    // elemsG2[0] = -g2
    ep2_new(&elemsG2[0]);
    ep2_neg(elemsG2[0], core_get()->ep2_g); // could be hardcoded 

    // map all hashes to G1
    int offset = 0;
    for (int i=1; i < nb_hashes+1; i++) {
        // elemsG1[i] = h
        ep_new(elemsG1[i]);
        // hash to G1 
        map_to_G1(elemsG1[i], &hashes[offset], len_hashes[i-1]); 
        offset += len_hashes[i-1];
    }

    // aggregate public keys mapping to the same hash
    offset = 0;
    for (int i=1; i < nb_hashes+1; i++) {
        // elemsG2[i] = agg_pk[i]
        ep2_new(elemsG2[i]);
        ep2_sum_vector(elemsG2[i], (ep2_st*) &pks[offset] , pks_per_hash[i-1]);
        offset += pks_per_hash[i-1];
    }

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), nb_hashes+1);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

    for (int i=0; i < nb_hashes+1; i++) {
        ep_free(elemsG1[i]);
        ep2_free(elemsG2[i]);
    }
    free(elemsG1);
    free(elemsG2);
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) {
        return VALID;
    }
    return INVALID;
}


// Verifies the validity of an aggregated BLS signature under distinct public keys.
//
// Each key is mapped to a set of messages, so that the verification equation is 
// optimized to compute one pairing per public key. 
// - nb_pks is the number of the public keys in the map.
// - pks is pointer to all pks in order where the key at index i
//   is mapped to hashes_per_pk[i] hashes. 
// - the messages (hashes) are flattened in hashes in the same public key order,
//  each with a length in len_hashes.
//
// membership check of the signature in G1 is verified in this function
// membership check of pks in G2 is not verified in this function
// the membership check is separated to allow optimizing multiple verifications using the same pks
int bls_verifyPerDistinctKey(const byte* sig, 
                         const int nb_pks, const ep2_st* pks, const uint32_t* hashes_per_pk,
                         const byte* hashes, const uint32_t* len_hashes){

    
    ep_t* elemsG1 = (ep_t*)malloc((nb_pks + 1) * sizeof(ep_t));
    ep2_t* elemsG2 = (ep2_t*)malloc((nb_pks + 1) * sizeof(ep2_t));

    // elemsG1[0] = s
    ep_new(elemsG1[0]);
    int read_ret = ep_read_bin_compact(elemsG1[0], sig, SIGNATURE_LEN);
    if (read_ret != RLC_OK) 
        return read_ret;

    // check s is on curve and in G1
    if (check_membership_G1(elemsG1[0]) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID;

    // elemsG2[0] = -g2
    ep2_new(&elemsG2[0]);
    ep2_neg(elemsG2[0], core_get()->ep2_g); // could be hardcoded 

    // set the public keys
    for (int i=1; i < nb_pks+1; i++) {
        ep2_new(elemsG2[i]);
        ep2_copy(elemsG2[i], (ep2_st*) &pks[i-1]);
    }

    // map all hashes to G1 and aggregate the ones with the same public key
    int data_offset = 0;
    int index_offset = 0;
    for (int i=1; i < nb_pks+1; i++) {
        // array for all the hashes under the same key
        ep_st* h_array = (ep_st*)malloc(hashes_per_pk[i-1] * sizeof(ep_st));
        for (int j=0; j < hashes_per_pk[i-1]; j++) {
            ep_new(&h_array[j]);
            // map the hash to G1
            map_to_G1(&h_array[j], &hashes[data_offset], len_hashes[index_offset]); 
            data_offset += len_hashes[index_offset];
            index_offset++; 
        }
        // aggregate all the points of the array
        ep_new(elemsG1[i]);   
        ep_sum_vector(elemsG1[i], h_array, hashes_per_pk[i-1]);

        // free the array
        for (int j=0; j < hashes_per_pk[i-1]; j++) ep_free(h_array[j]);
        free(h_array);
    }

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), nb_pks+1);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

    for (int i=0; i < nb_pks+1; i++) {
        ep_free(elemsG1[i]);
        ep2_free(elemsG2[i]);
    }
    free(elemsG1);
    free(elemsG2);
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) {
        return VALID;
    }
    return INVALID;
}

// Verifies a BLS signature in a byte buffer.
// membership check of the signature in G1 is verified.
// membership check of pk in G2 is not verified in this function.
// the membership check in G2 is separated to allow optimizing multiple verifications using the same key.
int bls_verify(const ep2_t pk, const byte* sig, const byte* data, const int len) {  
    ep_t s;
    ep_new(s);
    
    // deserialize the signature
    int read_ret = ep_read_bin_compact(s, sig, SIGNATURE_LEN);
    if (read_ret != RLC_OK) 
        return read_ret;

    // check s is on curve and in G1
    if (check_membership_G1(s) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID;
    
    return bls_verify_ep(pk, s, data, len);
}

// binary tree structure to be used by bls_batch verify.
// Each node contains a signature and a public key, the signature (resp. the public key) 
// being the aggregated signature of the two children's signature (resp. public keys).
// The leaves contain the initial signatures and public keys.
typedef struct st_node { 
    ep_st* sig;
    ep2_st* pk;  
    struct st_node* left; 
    struct st_node* right; 
} node;

static node* new_node(const ep2_st* pk, const ep_st* sig){
    node* t = (node*) malloc(sizeof(node));
    t->pk = (ep2_st*)pk;
    t->sig = (ep_st*)sig;
    t->right = t->left = NULL;
    return t;
}

// builds a binary tree of aggregation of signatures and public keys recursively.
static node* build_tree(const int len, const ep2_st* pks, const ep_st* sigs) {
    // check if a leaf is reached
    if (len == 1) {
        return new_node(pks, sigs);  // use the first element of the arrays
    }

    // a leaf is not reached yet, 
    int right_len = len/2;
    int left_len = len - right_len;

    // create a new node with new points
    node* t = new_node((ep2_st*)malloc(sizeof(ep2_st)), (ep_st*)malloc(sizeof(ep_st)));
    ep_new(t->sig);
    ep_new(t->pk);
    // build the tree in a top-down way
    t->left = build_tree(left_len, &pks[0], &sigs[0]);
    t->right = build_tree(right_len, &pks[left_len], &sigs[left_len]);
    // sum the children
    ep_add_jacob(t->sig, t->left->sig, t->right->sig);
    ep2_add_projc(t->pk, t->left->pk, t->right->pk); 
    return t;
}

static void free_tree(node* root) {
    // relic free
    ep_free(root->sig);
    ep2_free(root->pk);
    if (root->left) {
        // only free non-leaves, leaves are allocated as an entire array
        free(root->sig);
        free(root->pk);
        // free the children nodes
        free_tree(root->left);
        free_tree(root->right);
    }
    free(root);
}

// verify the binary tree and fill the results using recursive batch verifications.
static void bls_batchVerify_tree(const node* root, const int len, byte* results, 
        const byte* data, const int data_len) {

    // verify the aggregated signature against the aggregated public key.
    int res =  bls_verify_ep(root->pk, root->sig, data, data_len);

    // if the result is valid, all the subtree signatures are valid.
    if (res == VALID) {
        for (int i=0; i < len; i++) {
            if (results[i] == UNDEFINED) results[i] = VALID; // do not overwrite invalid results
        }
        return;
    }

    // check if root is a leaf
    if (root->left == NULL) { // no need to check the right side
        *results = INVALID;
        return;
    }

    // otherwise, at least one of the subtree signatures is invalid. 
    // use the binary tree structure to find the invalid signatures. 
    int right_len = len/2;
    int left_len = len - right_len;
    bls_batchVerify_tree(root->left, left_len, &results[0], data, data_len);
    bls_batchVerify_tree(root->right, right_len, &results[left_len], data, data_len);
}

// Batch verifies the validity of a multiple BLS signatures of the 
// same message under multiple public keys.
//
// - membership checks of all signatures is verified upfront.
// - use random coefficients for signatures and public keys at the same index.
// - optimize the verification by verifying an aggregated signature against an aggregated
//  public key, and use a recursive verification to find invalid signatures.  
void bls_batchVerify(const int sigs_len, byte* results, const ep2_st* pks_input,
     const byte* sigs_bytes, const byte* data, const int data_len) {  

    // initialize results to undefined
    memset(results, UNDEFINED, sigs_len);
    
    // build the arrays of G1 and G2 elements to verify
    ep2_st* pks = (ep2_st*) malloc(sigs_len * sizeof(ep2_st));
    ep_st* sigs = (ep_st*) malloc(sigs_len * sizeof(ep_st));
    bn_t r;
    for (int i=0; i < sigs_len; i++) {
        ep_new(sigs[i]);
        ep2_new(pks[i]);
        // convert the signature points:
        // - invalid points are stored as infinity points with an invalid result, so that
        // the tree aggregations remain valid.
        // - valid points are multiplied by a random scalar (same for public keys at same index)
        // to make sure a signature at index (i) is verified against the public key at the same index.
        int read_ret = ep_read_bin_compact(&sigs[i], &sigs_bytes[SIGNATURE_LEN*i], SIGNATURE_LEN);
        if ( read_ret != RLC_OK || check_membership_G1(&sigs[i]) != VALID) {
            if (read_ret == UNDEFINED) // unexpected error case 
                return;
            // set signature as infinity and set result as invald
            ep_set_infty(&sigs[i]);
            ep2_copy(&pks[i], (ep2_st*) &pks_input[i]);
            results[i] = INVALID;
        // multiply signatures and public keys at the same index by random coefficients
        } else {
            // random non-zero coefficient of a least 128 bits
            bn_rand(r, RLC_POS, SEC_BITS);
            bn_add_dig(r, r, 1); 
            ep_mul_lwnaf(&sigs[i], &sigs[i], r);
            ep2_mul_lwnaf(&pks[i], (ep2_st*) &pks_input[i], r);      
        }
    }

    // build a binary tree of aggreagtions
    node* root = build_tree(sigs_len, &pks[0], &sigs[0]);

    // verify the binary tree and fill the results using batch verification
    bls_batchVerify_tree(root, sigs_len, &results[0], data, data_len);

    // free the allocated memory 
    free_tree(root); // (relic free is called in free_tree)
    free(sigs); 
    free(pks);
    bn_free(r);   
}
