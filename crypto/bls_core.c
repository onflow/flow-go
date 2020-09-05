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

// checks an input scalar is less than the groups order (r)
int check_membership_Zr(const bn_t a){
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);
    int res = bn_cmp(a,r);
    if (res == RLC_LT) return VALID;
    return INVALID;
}

// checks if input point s is on the curve E1 
// and is in the subgroup G1
// membership check in G1 is using a naive scalar multiplication by the group order
// TODO: switch to the faster Bowe check 
static int check_membership_G1(const ep_t p){
#if MEMBERSHIP_CHECK
    // check p is on curve
    if (!ep_is_valid(p))
        return INVALID;
    // check p is in G1
    ep_t inf;
    ep_new(inf);
    // check p^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    // TODO : write a simple lwnaf without reduction
    ep_mul_basic(inf, p, &core_get()->ep_r);
    if (!ep_is_infty(inf)){
        ep_free(inf);
        return INVALID;
    }
    ep_free(inf);
#endif
    return VALID;
}

// checks if input point s is on the curve E2 
// and is in the subgroup G2
// membership check in G2 is using a naive scalar multiplication by the group order
// TODO: switch to the faster Bowe check 
int check_membership_G2(const ep2_t p){
#if MEMBERSHIP_CHECK
    // check p is on curve
    if (!ep2_is_valid((ep2_st*)p))
        return INVALID;
    // check p is in G2
    ep2_t inf;
    ep2_new(inf);
    // check p^order == infinity
    // use basic double & add as lwnaf reduces the expo modulo r
    // TODO : write a simple lwnaf without reduction
    ep2_mul_basic(inf, (ep2_st*)p, &core_get()->ep_r);
    if (!ep2_is_infty(inf)){
        ep2_free(inf);
        return INVALID;
    }
    ep2_free(inf);
#endif
    return VALID;
}

// Computes a BLS signature
void bls_sign(byte* s, const bn_t sk, const byte* data, const int len) {
    ep_t h;
    ep_new(h);
    // hash to G1
    map_to_G1(h, data, len);
    // s = h^sk
	ep_mult(h, h, sk);  
    ep_write_bin_compact(s, h, SIGNATURE_LEN);
    ep_free(h);
}

// Verifies a BLS signature in a G1 point.
// membership check of the signature in G1 is verified in this function
// membership check of pk in G2 is not verified in this function
// the membership check is separated to allow optimizing multiple verifications using the same key.
static int bls_verify_ep(const ep2_t pk, const ep_t s, const byte* data, const int len) { 
    ep_t elemsG1[2];
    ep2_t elemsG2[2];

    // elemsG1[0] = s
    ep_new(elemsG1[0]);
    ep_copy(elemsG1[0], (ep_st*)s);

    // check s is on curve and in G1
    if (check_membership_G1(elemsG1[0]) != VALID) // only enabled if MEMBERSHIP_CHECK==1
        return INVALID;

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
    ep2_neg(elemsG2[0], &core_get()->ep2_g); // could be hardcoded 

    fp12_t pair;
    fp12_new(&pair);
    // double pairing with Optimal Ate 
    pp_map_sim_oatep_k12(pair, (ep_t*)(elemsG1) , (ep2_t*)(elemsG2), 2);

    // compare the result to 1
    int res = fp12_cmp_dig(pair, 1);

#elif SINGLE_PAIRING   
    fp12_t pair1, pair2;
    fp12_new(&pair1); fp12_new(&pair2);
    pp_map_oatep_k12(pair1, elemsG1[0], &core_get()->ep2_g);
    pp_map_oatep_k12(pair2, elemsG1[1], elemsG2[1]);

    int res = fp12_cmp(pair1, pair2);
#endif
    fp12_free(&one);
    ep_free(elemsG1[0]);
    ep_free(elemsG1[1]);
    ep2_free(elemsG2[0]);
    ep2_free(elemsG2[1]);
    
    if (res == RLC_EQ && core_get()->code == RLC_OK) 
        return VALID;
    else 
        return INVALID;
}

// Verifies a BLS signature in a byte buffer.
// membership check of the signature in G1 is verified in this function
// membership check of pk in G2 is not verified in this function
// the membership check is separated to allow optimizing multiple verifications using the same key.
int bls_verify(const ep2_t pk, const byte* sig, const byte* data, const int len) {  
    ep_t s;
    ep_new(s);
    if (ep_read_bin_compact(s, sig, SIGNATURE_LEN) != RLC_OK) 
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

// builds a binary tree of aggregation of signatures and public keys
static node* build_tree(const int len, const ep2_st* pks, const ep_st* sigs) {
    // check if a leave is reached
    if (len == 1) {
        return new_node(pks, sigs);  // use the first element of the arrays
    }

    // a leave is not reached yet, 
    int right_len = len/2;
    int left_len = len - right_len;

    // create a new node with new points
    node* t = new_node((ep2_st*)malloc(sizeof(ep2_st)), (ep_st*)malloc(sizeof(ep_st)));
    ep_new(t->sig);
    ep_new(t->pk);
    // build the tree in a top-down way
    t->left = build_tree(left_len, pks, sigs);
    t->right = build_tree(right_len, pks + left_len, sigs + left_len);
    // sum the children
    ep_add_projc(t->sig, t->left->sig, t->right->sig);
    ep2_add_projc(t->pk, t->left->pk, t->right->pk);  
    return t;
}

// Batch verifies the validity of a multiple BLS signatures of the 
// same message under multiple public keys.
void bls_batchVerify(const int sigs_len, byte* results, const ep2_st* pks,
     const byte* sigs_bytes, const byte* data, const int data_len) {  

    // convert the signature points
    ep_st* sigs = (ep_st*) malloc(sigs_len * sizeof(ep_st));
    for (int i=0; i < sigs_len; i++) {
        ep_new(sigs[i]);
        if (ep_read_bin_compact(&sigs[i], &sigs_bytes[SIGNATURE_LEN*i], SIGNATURE_LEN) != RLC_OK) {
            // set a signature that doesn't verify with the public key
            if (ep2_is_infty((ep2_st*)&pks[i])) 
                ep_set_infty(&sigs[i]);
            else 
                ep_curve_get_gen(&sigs[i]);
            }
    }

    // build a binary tree of aggreagtions
    node* root = build_tree(sigs_len, pks, sigs);
    int res =  bls_verify_ep(root->pk, root->sig, data, data_len);
    for (int i=0; i < sigs_len; i++) results[i] = res;
}
