// +build relic

#include "misc.h"
#include "bls_include.h"


// DEBUG related functions
void _bytes_print(char* s, byte* data, int len) {
    printf("[%s]:\n", s);
    for (int i=0; i<len; i++) 
        printf("%02x,", data[i]);
    printf("\n");
}

void _fp_print(char* s, fp_st* a) {
    char* str = malloc(sizeof(char) * fp_size_str(*a, 16));
    fp_write_str(str, 100, *a, 16);
    printf("[%s]:\n%s\n", s, str);
    free(str);
}

void _bn_print(char* s, bn_st *a) {
    char* str = malloc(sizeof(char) * bn_size_str(a, 16));
    bn_write_str(str, 100, a, 16);
    printf("[%s]:\n%s\n", s, str);
    free(str);
}

void _ep_print(char* s, ep_st* p) {
    printf("[%s]:\n", s);
    g1_print(p);
}

void _ep2_print(char* s, ep2_st* p) {
    printf("[%s]:\n", s);
    g2_print(p);
}

// seeds relic PRG
void _seed_relic(byte* seed, int len) {
    #if RAND == HASHD
    // instantiate a new DRBG
    ctx_t *ctx = core_get();
    ctx->seeded = 0;
    #endif
    rand_seed(seed, len);
}

// generates a random number less than the order r
void _bn_randZr(bn_t x) {
    bn_t r;
    bn_new(r); 
    g2_get_ord(r);

    bn_new_size(x, bn_size_raw(r));
    if (x)
        bn_rand_mod(x,r);
    bn_free(r);
}

// ep_write_bin_compact exports a point a in E(Fp) to a buffer bin in a compressed or uncompressed form.
// len is the allocated size of the buffer bin for sanity check
// The encoding is inspired from zkcrypto (https://github.com/zkcrypto/pairing/tree/master/src/bls12_381) with a small change to accomodate Relic lib
// The code is a modified version of Relic ep_write_bin
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void _ep_write_bin_compact(byte *bin, const ep_st *a, const int len) {
    ep_t t;
    ep_null(t);
    const int G1size = (G1_BYTES/(SERIALIZATION+1));

    if (len!=G1size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep_is_infty(a)) {
            // set the infinity bit
            bin[0] = (SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G1size-1);
            return;
    }

    TRY {
        ep_new(t);
        ep_norm(t, a);
        fp_write_bin(bin, Fp_BYTES, t->x);

        if (SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_bit(t->y, 0) << 5);
        } else {
            fp_write_bin(bin + Fp_BYTES, Fp_BYTES, t->y);
        }
    } CATCH_ANY {
        THROW(ERR_CAUGHT);
    }

    bin[0] |= (SERIALIZATION << 7);
    ep_free(t);
 }


// ep_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// len is the size of the input buffer
// The encoding is inspired from zkcrypto (https://github.com/zkcrypto/pairing/tree/master/src/bls12_381) with a small change to accomodate Relic lib
// The code is a modified version of Relic ep_write_bin
void _ep_read_bin_compact(ep_st* a, const byte *bin, const int len) {
    const int G1size = (G1_BYTES/(SERIALIZATION+1));
    if (len!=G1size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
    // check if the point is infinity
    if (bin[0] & 0x40) {
        // check if the remaining bits are cleared
        if (bin[0] & 0x3F) {
            THROW(ERR_NO_VALID);
            return;
        }
        for (int i=1; i<G1size-1; i++) {
            if (bin[i]) {
                THROW(ERR_NO_VALID);
                return;
            } 
        }
		ep_set_infty(a);
		return;
	} 

    int compressed = bin[0] >> 7;
    int y_is_odd = (bin[0] >> 5) & 1;

    if (y_is_odd && (!compressed)) {
        THROW(ERR_NO_VALID);
        return;
    } 

	a->norm = 1;
	fp_set_dig(a->z, 1);
    byte* temp = (byte*)malloc(Fp_BYTES);
    if (!temp) {
        THROW(ERR_NO_MEMORY);
        return;
    }
    memcpy(temp, bin, Fp_BYTES);
    temp[0] &= 0x1F;
	fp_read_bin(a->x, temp, Fp_BYTES);
    free(temp);

    if (SERIALIZATION == UNCOMPRESSED) {
        fp_read_bin(a->y, bin + Fp_BYTES, Fp_BYTES);
    }
    else {
        fp_zero(a->y);
        fp_set_bit(a->y, 0, y_is_odd);
        ep_upk(a, a);
    }
}

// _ep2_write_bin_compact exports a point in E(Fp^2) to a buffer in a compressed or uncompressed form.
// The code is a modified version of Relic ep2_write_bin
// The most significant bit of the buffer, when set, indicates that the point is in compressed form. 
// Otherwise, the point is in uncompressed form.
// The second-most significant bit indicates that the point is at infinity. 
// If this bit is set, the remaining bits of the group element's encoding should be set to zero.
// The third-most significant bit is set if (and only if) this point is in compressed form and it is not the point at infinity and its y-coordinate is odd.
void _ep2_write_bin_compact(byte *bin, const ep2_st *a, const int len) {
    ep2_t t;
    ep2_null(t);
    const int G2size = (G2_BYTES/(SERIALIZATION+1));

    if (len!=G2size) {
        THROW(ERR_NO_BUFFER);
        return;
    }
 
    if (ep2_is_infty((ep2_st *)a)) {
            // set the infinity bit
            bin[0] = (SERIALIZATION << 7) | 0x40;
            memset(bin+1, 0, G2size-1);
            return;
    }

    TRY {
        ep2_new(t);
        ep2_norm(t, (ep2_st *)a);
        fp2_write_bin(bin, 2*Fp_BYTES, t->x, 0);

        if (SERIALIZATION == COMPRESSED) {
            bin[0] |= (fp_get_bit(t->y[0], 0) << 5);
        } else {
            fp2_write_bin(bin + 2*Fp_BYTES, 2*Fp_BYTES, t->y, 0);
        }
    } CATCH_ANY {
        THROW(ERR_CAUGHT);
    }

    bin[0] |= (SERIALIZATION << 7);
    ep_free(t);
}

// _ep2_read_bin_compact imports a point from a buffer in a compressed or uncompressed form.
// The code is a modified version of Relic ep_write_bin
void _ep2_read_bin_compact(ep2_st* a, const byte *bin, const int len) {
    const int G2size = (G2_BYTES/(SERIALIZATION+1));
    if (len!=G2size) {
        THROW(ERR_NO_BUFFER);
        return;
    }

    // check if the point in infinity
    if (bin[0] & 0x40) {
        // the remaining bits need to be cleared
        if (bin[0] & 0x3F) {
            THROW(ERR_NO_VALID);
            return;
        }
        for (int i=1; i<G2size-1; i++) {
            if (bin[i]) {
                THROW(ERR_NO_VALID);
                return;
            } 
        }
		ep2_set_infty(a);
		return;
	} 
    byte compressed = bin[0] >> 7;
    byte y_is_odd = (bin[0] >> 5) & 1;
    if (y_is_odd && (!compressed)) {
        THROW(ERR_NO_VALID);
        return;
    } 
	a->norm = 1;
	fp_set_dig(a->z[0], 1);
	fp_zero(a->z[1]);
    byte* temp = (byte*)malloc(2*Fp_BYTES);
    if (!temp) {
        THROW(ERR_NO_MEMORY);
        return;
    }
    memcpy(temp, bin, 2*Fp_BYTES);
    // clear the header bits
    temp[0] &= 0x1F;
    fp2_read_bin(a->x, temp, 2*Fp_BYTES);
    free(temp);


    if (SERIALIZATION == UNCOMPRESSED) {
        fp2_read_bin(a->y, bin + 2*Fp_BYTES, 2*Fp_BYTES);
    }
    else {
        fp2_zero(a->y);
        fp_set_bit(a->y[0], 0, y_is_odd);
		fp_zero(a->y[1]);
        ep2_upk(a, a);
    }
}

#if (hashToPoint == HASHCHECK)
// Simple hashing to G1 as described in the original BLS paper 
// https://www.iacr.org/archive/asiacrypt2001/22480516.pdf
// taken and modified from Relic library
void mapToG1_hashCheck(ep_t p, const uint8_t *msg, int len) {
	bn_t k, pm1o2;
	fp_t t;
	uint8_t digest[RLC_MD_LEN];

	bn_null(k);
	bn_null(pm1o2);
	fp_null(t);
	ep_null(q);

	TRY {
		bn_new(k);
		bn_new(pm1o2);
		fp_new(t);
		ep_new(q);

		pm1o2->sign = RLC_POS;
		pm1o2->used = RLC_FP_DIGS;
		dv_copy(pm1o2->dp, fp_prime_get(), RLC_FP_DIGS);
		bn_hlv(pm1o2, pm1o2);
		md_map(digest, msg, len);
		bn_read_bin(k, digest, RLC_MIN(RLC_FP_BYTES, RLC_MD_LEN));
		fp_prime_conv(t, k);
		fp_prime_back(k, t);

        fp_prime_conv(p->x, k);
        fp_zero(p->y);
        fp_set_dig(p->z, 1);

        while (1) {
            ep_rhs(t, p);
            if (fp_srt(p->y, t)) {
                p->norm = 1;
                break;
            }
            fp_add_dig(p->x, p->x, 1);
        }

        // Now, multiply by cofactor to get the correct group. 
        ep_curve_get_cof(k);
        if (bn_bits(k) < RLC_DIG) {
            ep_mul_dig(p, p, k->dp[0]);
        } else {
            ep_mul_basic(p, p, k);
        }
	}
	CATCH_ANY {
		THROW(ERR_CAUGHT);
	}
	FINALLY {
		bn_free(k);
		bn_free(pm1o2);
		fp_free(t);
		ep_free(q);
	}
}
#endif
//---------------------------

extern prec_st bls_prec;

#if (hashToPoint==OPSWU)
const uint8_t E_a1[47] = { 
    0x14, 0x46, 0x98, 0xa3, 0xb8, 0xe9, 0x43, 0x3d,
    0x69, 0x3a, 0x02, 0xc9, 0x6d, 0x49, 0x82, 0xb0,
    0xea, 0x98, 0x53, 0x83, 0xee, 0x66, 0xa8, 0xd8,
    0xe8, 0x98, 0x1a, 0xef, 0xd8, 0x81, 0xac, 0x98,
    0x93, 0x6f, 0x8d, 0xa0, 0xe0, 0xf9, 0x7f, 0x5c, 
    0xf4, 0x28, 0x08, 0x2d, 0x58, 0x4c, 0x1d,
    };


const uint8_t E_b1[48] = { 
    0x12, 0xe2, 0x90, 0x8d, 0x11, 0x68, 0x80, 0x30,
    0x01, 0x8b, 0x12, 0xe8, 0x75, 0x3e, 0xee, 0x3b,
    0x20, 0x16, 0xc1, 0xf0, 0xf2, 0x4f, 0x40, 0x70,
    0xa0, 0xb9, 0xc1, 0x4f, 0xce, 0xf3, 0x5e, 0xf5,
    0x5a, 0x23, 0x21, 0x5a, 0x31, 0x6c, 0xea, 0xa5,
    0xd1, 0xcc, 0x48, 0xe9, 0x8e, 0x17, 0x2b, 0xe0, 
    };

// check if (U/V) is a square, return 1 if yes, 0 otherwise 
// if 1 is returned, out contains sqrt(U/V)
// out should not be the same as U, or V
int quotient_sqrt(fp_t out, const fp_t u, const fp_t v) {
    fp_st tmp;
    fp_new(&tmp);

    fp_sqr(out, v);                  // V^2
    fp_mul(tmp, u, v);               // UV
    fp_mul(out, out, tmp);           // UV^3
    fp_exp(out, out, &bls_prec.p_3div4);        // (UV^3)^((p-3)/4)
    fp_mul(out, out, tmp);           // UV(UV^3)^((p-3)/4)

    fp_sqr(tmp, out);     // out^2
    fp_mul(tmp, tmp, v);  // out^2 * V
    fp_sub(tmp, tmp, u);   // out^2 * V - U

    fp_free(&tmp);
    return fp_cmp_dig(tmp, 0)==RLC_EQ;
}


// Maps the field element t to a point p in E1(Fp) where E1: y^2 = g(x) = x^3 + a1*x + b1 
// using optimized non-constant-time SWU impl
// p point is in Jacobian coordinates
static inline void mapToE1_swu(ep_st* p, const fp_t t) {
    const int tmp_len = 5;
    fp_t* fp_tmp = (fp_t*) malloc(tmp_len*sizeof(fp_t));
    for (int i=0; i<tmp_len; i++) fp_new(&fp_tmp[i]);

    // compute numerator and denominator of X0(t) = N / D
    fp_sqr(fp_tmp[0], t);                      // t^2
    fp_sqr(fp_tmp[1], fp_tmp[0]);             // t^4
    fp_sub(fp_tmp[1], fp_tmp[0], fp_tmp[1]);  // t^2 - t^4
    fp_set_dig(fp_tmp[3], 1);
    fp_sub(fp_tmp[2], fp_tmp[3], fp_tmp[1]);        // t^4 - t^2 + 1
    fp_mul(fp_tmp[2], fp_tmp[2], bls_prec.b1);     // N = b * (t^4 - t^2 + 1)                
    fp_mul(fp_tmp[1], fp_tmp[1], bls_prec.a1);     // D = a * (t^2 - t^4)                    
    if (fp_cmp_dig(fp_tmp[1], 0) == RLC_EQ) {
        // t was 0, -1, 1, so num is b and den is 0; set den to -a, because -b/a is square in Fp
        fp_neg_basic(fp_tmp[1], bls_prec.a1);
    }

    // compute numerator and denominator of g(X0(t)) = U / V 
    // U = N^3 + a1 * N * D^2 + b1 D^3
    // V = D^3
    fp_sqr(fp_tmp[3], fp_tmp[1]);              // D^2
    fp_mul(fp_tmp[4], fp_tmp[2], fp_tmp[3]);  // N * D^2
    fp_mul(fp_tmp[4], fp_tmp[4], bls_prec.a1);      // a * N * D^2
                                                   
    fp_mul(fp_tmp[3], fp_tmp[3], fp_tmp[1]);  // V = D^3
    fp_mul(fp_tmp[5], fp_tmp[3], bls_prec.b1);      // b1 * D^3
    fp_add(fp_tmp[4], fp_tmp[4], fp_tmp[5]);   // a1 * N * D^2 + b1 * D^3
                                                   
    fp_sqr(fp_tmp[5], fp_tmp[2]);              // N^2
    fp_mul(fp_tmp[5], fp_tmp[5], fp_tmp[2]);  // N^3
    fp_add(fp_tmp[4], fp_tmp[4], fp_tmp[5]);   // U

    // compute sqrt(U/V)
    if (!quotient_sqrt(fp_tmp[5], fp_tmp[4], fp_tmp[3])) {
        // g(X0(t)) was nonsquare, so convert to g(X1(t))
        // NOTE: multiplying by t^3 preserves sign of t, so no need to apply sgn0(t) to y
        fp_mul(fp_tmp[5], fp_tmp[5], fp_tmp[0]);  // t^2 * sqrtCand
        fp_mul(fp_tmp[5], fp_tmp[5], t);           // t^3 * sqrtCand
        fp_mul(fp_tmp[2], fp_tmp[2], fp_tmp[0]);  // b * t^2 * (t^4 - t^2 + 1)
        fp_neg_basic(fp_tmp[2], fp_tmp[2]);        // N = - b * t^2 * (t^4 - t^2 + 1)
    } else if (dv_cmp(bls_prec.p_1div2, t, FP_DIGITS) ==  RLC_LT) {
        // g(X0(t)) was square and t is negative, so negate y
        fp_neg_basic(fp_tmp[5], fp_tmp[5]);  // negate y because t is negative
    }

    // convert (x,y)=(N/D, y) into (X,Y,Z) where Z=D
    // Z = D, X = x*D^2 = N.D , Y = y*D^3
    fp_mul(p->x, fp_tmp[2], fp_tmp[1]);  // X = N*D
    fp_mul(p->y, fp_tmp[5], fp_tmp[3]);  // Y = y*D^3
    fp_copy(p->z, fp_tmp[1]);
    p->norm = 0;
    
    for (int i=0; i<tmp_len; i++) fp_free(&fp_tmp[i]);
    free(fp_tmp);
}

// 11-isogeny map
// computes the mapping or p and stores it in r
static inline void eval_iso11(ep_st* r, const ep_st*  p) {
    const int tmp_len = 5;
    fp_t* fp_tmp = (fp_t*) malloc(tmp_len*sizeof(fp_t));
    for (int i=0; i<tmp_len; i++) fp_new(&fp_tmp[i]);

    // precompute even powers of Z up to Z^30
    bint_sqr(bint_tmp[31], p->z);                 // Z^2
    bint_sqr(bint_tmp[30], bint_tmp[31]);                // Z^4
    bint_mul(bint_tmp[29], bint_tmp[30], bint_tmp[31]);  // Z^6
    bint_sqr(bint_tmp[28], bint_tmp[30]);                // Z^8
    for (unsigned i = 0; i < 3; ++i) {
        bint_mul(bint_tmp[27 - i], bint_tmp[28 - i], bint_tmp[31]);  // Z^10, Z^12, Z^14
    }
    bint_sqr(bint_tmp[24], bint_tmp[28]);  // Z^16
    for (unsigned i = 0; i < 7; ++i) {
        bint_mul(bint_tmp[23 - i], bint_tmp[24 - i], bint_tmp[31]);  // Z^18, ..., Z^30
    }

    // Ymap denominator
    compute_map_zvals(iso_yden, bint_tmp + 17, ELLP_YMAP_DEN_LEN);         // k_(15-i) Z^(2i)
    bint_add(bint_tmp[16], p->x, bint_tmp[ELLP_YMAP_DEN_LEN - 1]);  // X + k_14 Z^2 (denom is monic)
    bint_horner(bint_tmp[16], p->x, ELLP_YMAP_DEN_LEN - 2);         // Horner for rest
    bint_mul(bint_tmp[15], bint_tmp[16], bint_tmp[31]);                    // Yden * Z^2
    bint_mul(bint_tmp[15], bint_tmp[15], p->z);                     // Yden * Z^3

    // Ymap numerator
    compute_map_zvals(iso_ynum, bint_tmp + 17, ELLP_YMAP_NUM_LEN - 1);      // k_(15-i) Z^(2i)
    bint_mul(bint_tmp[16], p->x, iso_ynum[ELLP_YMAP_NUM_LEN - 1]);   // k_15 * X
    bint_add(bint_tmp[16], bint_tmp[16], bint_tmp[ELLP_YMAP_NUM_LEN - 2]);  // k_15 * X + k_14 Z^2
    bint_horner(bint_tmp[16], p->x, ELLP_YMAP_NUM_LEN - 3);          // Horner for rest
    bint_mul(bint_tmp[16], bint_tmp[16], p->y);                      // Ynum * Y
    // at this point, ymap num/den are in bint_tmp[16]/bint_tmp[15]

    // Xmap denominator
    compute_map_zvals(iso_xden, bint_tmp + 22, ELLP_XMAP_DEN_LEN);         // k_(10-i) Z^(2i)
    bint_add(bint_tmp[14], p->x, bint_tmp[ELLP_XMAP_DEN_LEN - 1]);  // X + k_9 Z^2 (denom is monic)
    bint_horner(bint_tmp[14], p->x, ELLP_XMAP_DEN_LEN - 2);         // Horner for rest
    // mul by Z^2 because numerator has degree one greater than denominator
    bint_mul(bint_tmp[14], bint_tmp[14], bint_tmp[31]);

    // Xmap numerator
    compute_map_zvals(iso_xnum, bint_tmp + 21, ELLP_XMAP_NUM_LEN - 1);      // k_(11-i) Z^(2i)
    bint_mul(bint_tmp[13], p->x, iso_xnum[ELLP_XMAP_NUM_LEN - 1]);   // k_11 * X
    bint_add(bint_tmp[13], bint_tmp[13], bint_tmp[ELLP_XMAP_NUM_LEN - 2]);  // k_11 * X + k_10 * Z^2
    bint_horner(bint_tmp[13], p->x, ELLP_XMAP_NUM_LEN - 3);          // Horner for rest

    // at this point, xmap num/den are in bint_tmp[13]/bint_tmp[14]
    // now compute Jacobian projective coordinates
    bint_mul(r->z, bint_tmp[14], bint_tmp[15]);  // Zout = Xden Yden
    bint_mul(r->x, bint_tmp[13], bint_tmp[15]);  // Xnum Yden
    bint_mul(r->x, r->x, r->z);    // Xnum Xden Yden^2 = Xout => Xout / Zout^2 = Xnum / Xden
    bint_sqr(bint_tmp[12], r->z);                // Zout^2
    bint_mul(r->y, bint_tmp[16], bint_tmp[14]);  // Ynum Xden
    bint_mul(r->y, r->y, bint_tmp[12]);   // Ynum Xden Zout^2 = Yout => Yout / Zout^3 = Ynum / Yden
    
    for (int i=0; i<tmp_len; i++) fp_free(&fp_tmp[i]);
    free(fp_tmp);
}



// evaluate the SWU map twice, add results together, apply isogeny map, clear cofactor
static void mapToG1_opswu(ep_st* p, const uint8_t *msg, int len) {
    fp_st t1, t2;
    fp_read_bin(t1, msg, len/2);
    fp_read_bin(t2, msg + len/2, len - len/2);
    _fp_print("t1", &t1);
    _fp_print("t2", &t2);

    ep_st p_temp;
    ep_new(&p_temp);
    mapToE1_swu(&p_temp, t1);
    eval_iso11(&p_temp, p_temp); // map to E
    ep_norm(&p_temp,&p_temp);
    _ep_print("hash to point", &p_temp);
    mapToE1_swu(p, t2);
    eval_iso11(p,p); 
    ep_norm(p,p);
    _ep_print("hash to point", p);

    //ep_add_projc(p, p, &p_temp);
    //eval_iso11(&p_temp, p); // map to E

    //clear_h_chain(p, &p_temp); // map to G1: expo by z-1 
    //ep_norm(p, p);  // normalize ??
    ep_free(&p_temp);
}
#endif

// computes hashing to G1 
// DEBUG/test function
ep_st* _hashToG1(const byte* data, const int len) {
    ep_st* h = (ep_st*) malloc(sizeof(ep_st));
    ep_new(h);
    // hash to G1 (construction 2 in https://eprint.iacr.org/2019/403.pdf)
    #if hashToPoint==OPSWU
    mapToG1_opswu(h, data, len);
    #elif hashToPoint==SWU
    mapToG1_swu(h, data, len);
    #elif hashToPoint==HASHCHECK 
    mapToG1_hashCheck(h, data, len);
    #endif
    //_ep_print("hash to point", h);
    return h;
}

