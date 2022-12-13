// +build relic

#include "bls12381_utils.h"
#include "bls_include.h"

extern prec_st* bls_prec;

#if (hashToPoint== LOCAL_SSWU)

// These constants are taken from https://github.com/kwantam/bls12-381_hash 
// and converted to the Mongtomery domain. 
// Copyright 2019 Riad S. Wahby
const uint64_t iso_Nx_data[ELLP_Nx_LEN][Fp_DIGITS] = {
    {0x4d18b6f3af00131c, 0x19fa219793fee28c, 0x3f2885f1467f19ae,
     0x23dcea34f2ffb304, 0xd15b58d2ffc00054, 0x0913be200a20bef4,},
    {0x898985385cdbbd8b, 0x3c79e43cc7d966aa, 0x1597e193f4cd233a,
     0x8637ef1e4d6623ad, 0x11b22deed20d827b, 0x07097bc5998784ad,},
    {0xa542583a480b664b, 0xfc7169c026e568c6, 0x5ba2ef314ed8b5a6,
     0x5b5491c05102f0e7, 0xdf6e99707d2a0079, 0x0784151ed7605524,},
    {0x494e212870f72741, 0xab9be52fbda43021, 0x26f5577994e34c3d,
     0x049dfee82aefbd60, 0x65dadd7828505289, 0x0e93d431ea011aeb,},
    {0x90ee774bd6a74d45, 0x7ada1c8a41bfb185, 0x0f1a8953b325f464,
     0x104c24211be4805c, 0x169139d319ea7a8f, 0x09f20ead8e532bf6,},
    {0x6ddd93e2f43626b7, 0xa5482c9aa1ccd7bd, 0x143245631883f4bd,
     0x2e0a94ccf77ec0db, 0xb0282d480e56489f, 0x18f4bfcbb4368929,},
    {0x23c5f0c953402dfd, 0x7a43ff6958ce4fe9, 0x2c390d3d2da5df63,
     0xd0df5c98e1f9d70f, 0xffd89869a572b297, 0x1277ffc72f25e8fe,},
    {0x79f4f0490f06a8a6, 0x85f894a88030fd81, 0x12da3054b18b6410,
     0xe2a57f6505880d65, 0xbba074f260e400f1, 0x08b76279f621d028,},
    {0xe67245ba78d5b00b, 0x8456ba9a1f186475, 0x7888bff6e6b33bb4,
     0xe21585b9a30f86cb, 0x05a69cdcef55feee, 0x09e699dd9adfa5ac,},
    {0x0de5c357bff57107, 0x0a0db4ae6b1a10b2, 0xe256bb67b3b3cd8d,
     0x8ad456574e9db24f, 0x0443915f50fd4179, 0x098c4bf7de8b6375,},
    {0xe6b0617e7dd929c7, 0xfe6e37d442537375, 0x1dafdeda137a489e,
     0xe4efd1ad3f767ceb, 0x4a51d8667f0fe1cf, 0x054fdf4bbf1d821c,},
    {0x72db2a50658d767b, 0x8abf91faa257b3d5, 0xe969d6833764ab47,
     0x464170142a1009eb, 0xb14f01aadb30be2f, 0x18ae6a856f40715d,},
};

const uint64_t iso_Ny_data[ELLP_Ny_LEN][Fp_DIGITS] = {
    {0x2b567ff3e2837267, 0x1d4d9e57b958a767, 0xce028fea04bd7373,
     0xcc31a30a0b6cd3df, 0x7d7b18a682692693, 0x0d300744d42a0310,},
    {0x99c2555fa542493f, 0xfe7f53cc4874f878, 0x5df0608b8f97608a,
     0x14e03832052b49c8, 0x706326a6957dd5a4, 0x0a8dadd9c2414555,},
    {0x13d942922a5cf63a, 0x357e33e36e261e7d, 0xcf05a27c8456088d,
     0x0000bd1de7ba50f0, 0x83d0c7532f8c1fde, 0x13f70bf38bbf2905,},
    {0x5c57fd95bfafbdbb, 0x28a359a65e541707, 0x3983ceb4f6360b6d,
     0xafe19ff6f97e6d53, 0xb3468f4550192bf7, 0x0bb6cde49d8ba257,},
    {0x590b62c7ff8a513f, 0x314b4ce372cacefd, 0x6bef32ce94b8a800,
     0x6ddf84a095713d5f, 0x64eace4cb0982191, 0x0386213c651b888d,},
    {0xa5310a31111bbcdd, 0xa14ac0f5da148982, 0xf9ad9cc95423d2e9,
     0xaa6ec095283ee4a7, 0xcf5b1f022e1c9107, 0x01fddf5aed881793,},
    {0x65a572b0d7a7d950, 0xe25c2d8183473a19, 0xc2fcebe7cb877dbd,
     0x05b2d36c769a89b0, 0xba12961be86e9efb, 0x07eb1b29c1dfde1f,},
    {0x93e09572f7c4cd24, 0x364e929076795091, 0x8569467e68af51b5,
     0xa47da89439f5340f, 0xf4fa918082e44d64, 0x0ad52ba3e6695a79,},
    {0x911429844e0d5f54, 0xd03f51a3516bb233, 0x3d587e5640536e66,
     0xfa86d2a3a9a73482, 0xa90ed5adf1ed5537, 0x149c9c326a5e7393,},
    {0x462bbeb03c12921a, 0xdc9af5fa0a274a17, 0x9a558ebde836ebed,
     0x649ef8f11a4fae46, 0x8100e1652b3cdc62, 0x1862bd62c291dacb,},
    {0x05c9b8ca89f12c26, 0x0194160fa9b9ac4f, 0x6a643d5a6879fa2c,
     0x14665bdd8846e19d, 0xbb1d0d53af3ff6bf, 0x12c7e1c3b28962e5,},
    {0xb55ebf900b8a3e17, 0xfedc77ec1a9201c4, 0x1f07db10ea1a4df4,
     0x0dfbd15dc41a594d, 0x389547f2334a5391, 0x02419f98165871a4,},
    {0xb416af000745fc20, 0x8e563e9d1ea6d0f5, 0x7c763e17763a0652,
     0x01458ef0159ebbef, 0x8346fe421f96bb13, 0x0d2d7b829ce324d2,},
    {0x93096bb538d64615, 0x6f2a2619951d823a, 0x8f66b3ea59514fa4,
     0xf563e63704f7092f, 0x724b136c4cf2d9fa, 0x046959cfcfd0bf49,},
    {0xea748d4b6e405346, 0x91e9079c2c02d58f, 0x41064965946d9b59,
     0xa06731f1d2bbe1ee, 0x07f897e267a33f1b, 0x1017290919210e5f,},
    {0x872aa6c17d985097, 0xeecc53161264562a, 0x07afe37afff55002,
     0x54759078e5be6838, 0xc4b92d15db8acca8, 0x106d87d1b51d13b9,},
};

// sqrt_ration optimized for p mod 4 = 3.
// Check if (U/V) is a square, return 1 if yes, 0 otherwise 
// If 1 is returned, out contains sqrt(U/V),
// otherwise out is sqrt(z*U/V)
// out should not be the same as U, or V
static int sqrt_ratio_3mod4(fp_t out, const fp_t u, const fp_t v) {
    fp_t t0, t1, t2;

    fp_sqr(t1, v);                               // V^2
    fp_mul(t2, u, v);                            // U*V
    fp_mul(t1, t1, t2);                          // U*V^3
    fp_exp(out, t1, &bls_prec->p_3div4);         // (U*V^3)^((p-3)/4)
    fp_mul(out, out, t2);                        // (U*V)*(U*V^3)^((p-3)/4) = U^((p+1)/4) * V^(3p-5)/4 

    fp_sqr(t0, out);     // out^2
    fp_mul(t0, t0, v);   // out^2 * V

    int res = 1;
    if (fp_cmp(t0, u) != RLC_EQ) {               // check whether U/V is a quadratic residue
        fp_mul(out, out, bls_prec->sqrt_z);      // sqrt(-z)*U*V(UV^3)^((p-3)/4)
        res = 0;
    }
    
    return res;
}

// returns 1 if input is odd and 0 if input is even
static int sign_0(const fp_t in) {
#if FP_RDC == MONTY
    bn_t tmp;
    fp_prime_back(tmp, in); // TODO: entire reduction may not be needed to get the parity
    return bn_is_even(tmp);
#endif
    return in[0]&1;
}

// Maps the field element t to a point p in E1(Fp) where E1: y^2 = g(x) = x^3 + a1*x + b1 
// using optimized non-constant-time Simplified SWU implementation (A.B = 0)
// Outout point p is in Jacobian coordinates to avoid extra inversions.
static inline void map_to_E1_osswu(ep_t p, const fp_t t) {
    fp_t t0, t1, t2, t3, t4;

    // get the isogeny map coefficients
    ctx_t* ctx = core_get();
    fp_t *a1 = &ctx->ep_iso.a;
    fp_t *b1 = &ctx->ep_iso.b;
    fp_t *z = &ctx->ep_map_u;

    // compute numerator and denominator of X0(t) = N / D
    fp_sqr(t1, t);                            // t^2
    fp_mul(t1, t1, *z);                       // z * t^2
    fp_sqr(t2, t1);                           // z^2 * t^4
    fp_add(t2, t2, t1);                       // z * t^2 + z^2 * t^4   
    fp_add(t3, t2, bls_prec->r);              // z * t^2 + z^2 * t^4 + 1
    fp_mul(t3, t3, *b1);                      // N = b * (z * t^2 + z^2 * t^4 + 1)
 
    if (fp_is_zero(t2)) {
        fp_copy(p->z, bls_prec->a1z);         // D = a * z
    } else {
        fp_mul(p->z, t2, bls_prec->minus_a1); // D = - a * (z * t^2 + z^2 * t^4)
    }

    // compute numerator and denominator of g(X0(t)) = U / V 
    // U = N^3 + a1 * N * D^2 + b1 * D^3
    // V = D^3
    fp_sqr(t2, t3);                        // N^2
    fp_sqr(t0, p->z);                      // D^2
    fp_mul(t4, *a1, t0);                   // a * D^2
    fp_add(t2, t4, t2);                    // N^2 + a * D^2
    fp_mul(t2, t3, t2);                    // N^3 + a * N * D^2
    fp_mul(t0, t0, p->z);                  // V  =  D^3
    fp_mul(t4, *b1, t0);                   // b * V = b * D^3
    fp_add(t2, t4, t2);                    // U = N^3 + a1 * N * D^2 + b1 * D^3

    // compute sqrt(U/V)
    int is_sqr = sqrt_ratio_3mod4(p->y, t2, t0);
    if (is_sqr) {
        fp_copy(p->x, t3);      // x = N
    } else {
        fp_mul(p->x, t1, t3);   // x = N * z * t^2
        fp_mul(t1, t1, t);      // z * t^3
        fp_mul(p->y, p->y, t1); // y = z * t^3 * sqrt(r * U/V) where r is 1 or map coefficient z
    }

    // negate y to be the same sign of t
    if (sign_0(t) != sign_0(p->y)) {
        fp_neg(p->y, p->y);   // -y
    }

    // convert (x/D, y) into Jacobian (X,Y,Z) where Z=D to avoid inversion.
    // Z = D, X = x/D * D^2 = x*D , Y = y*D^3  
    fp_mul(p->x, p->x, p->z);             // X = N*D
    fp_mul(p->y, p->y, t0);               // Y = y*D^3
    // p->z is already equal to D 
    p->coord = JACOB;
}

// This code is taken from https://github.com/kwantam/bls12-381_hash 
// and adapted to use Relic modular arithemtic.  
// Copyright 2019 Riad S. Wahby
static inline void hornerPolynomial(fp_t accumulator, const fp_t x, 
        const int start_val, const fp_t* fp_tmp) {
    for (int i = start_val; i >= 0; --i) {
        fp_mul(accumulator, accumulator, x);            // acc *= x 
        fp_add(accumulator, accumulator, fp_tmp[i]);    // acc += next_val 
    }
}

// This code is taken from https://github.com/kwantam/bls12-381_hash 
// and adapted to use Relic modular arithemtic.  
// Copyright 2019 Riad S. Wahby
static inline void compute_map_zvals(const fp_t inv[], fp_t zv[], 
        const unsigned len, fp_t* fp_tmp) {
    for (unsigned i = 0; i < len; ++i) {
        fp_mul(fp_tmp[i], inv[i], zv[i]);
    }
}

// 11-isogeny map
// computes the mapping of p and stores the result in r
//
// This code is taken from https://github.com/kwantam/bls12-381_hash 
// and adapted to use Relic modular arithemtic. The constant tables 
// iso_D and iso_N were converted to the Montgomery domain. 
//
// Copyright 2019 Riad S. Wahby
// Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at

//        http://www.apache.org/licenses/LICENSE-2.0

//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.
static inline void eval_iso11(ep_t r, const ep_t  p) {
    fp_t fp_tmp[32];

    // precompute even powers of Z up to Z^30 in fp_tmp[31]..fp_tmp[17]
    fp_sqr(fp_tmp[31], p->z);                       // Z^2
    fp_sqr(fp_tmp[30], fp_tmp[31]);                 // Z^4
    fp_mul(fp_tmp[29], fp_tmp[30], fp_tmp[31]);     // Z^6
    fp_sqr(fp_tmp[28], fp_tmp[30]);                 // Z^8
    fp_mul(fp_tmp[27], fp_tmp[28], fp_tmp[31]);     // Z^10
    fp_sqr(fp_tmp[26], fp_tmp[29]);                 // Z^12
    fp_mul(fp_tmp[25], fp_tmp[26], fp_tmp[31]);     // Z^14
    fp_sqr(fp_tmp[24], fp_tmp[28]);                 // Z^16
    fp_mul(fp_tmp[23], fp_tmp[24], fp_tmp[31]);     // Z^18
    fp_sqr(fp_tmp[22], fp_tmp[27]);                 // Z^20
    fp_mul(fp_tmp[21], fp_tmp[22], fp_tmp[31]);     // Z^22
    fp_sqr(fp_tmp[20], fp_tmp[26]);                 // Z^24
    fp_mul(fp_tmp[19], fp_tmp[20], fp_tmp[31]);     // Z^26
    fp_sqr(fp_tmp[18], fp_tmp[25]);                 // Z^28
    fp_mul(fp_tmp[17], fp_tmp[18], fp_tmp[31]);     // Z^30

    // get isogeny map coefficients
    iso_t iso = ep_curve_get_iso();
    const int deg_dy = iso->deg_yd;
    const int deg_dx = iso->deg_xd;
    // TODO: get N coefficient from Relic and update N computations

    // y = Ny/Dy
    // compute Dy
    compute_map_zvals(iso->yd, fp_tmp + 17, deg_dy, fp_tmp);     // k_(15-i) Z^(2i)
    fp_add(fp_tmp[16], p->x, fp_tmp[deg_dy - 1]);        // X + k_14 Z^2 
    hornerPolynomial(fp_tmp[16], p->x, deg_dy - 2, fp_tmp);    // Horner for the rest
    fp_mul(fp_tmp[15], fp_tmp[16], fp_tmp[31]);                    // Dy * Z^2
    fp_mul(fp_tmp[15], fp_tmp[15], p->z);                           // Dy * Z^3

    // compute Ny
    compute_map_zvals(bls_prec->iso_Ny, fp_tmp + 17, ELLP_Ny_LEN - 1, fp_tmp); // k_(15-i) Z^(2i)
    fp_mul(fp_tmp[16], p->x, bls_prec->iso_Ny[ELLP_Ny_LEN - 1]);      // k_15 * X
    fp_add(fp_tmp[16], fp_tmp[16], fp_tmp[ELLP_Ny_LEN - 2]);  // k_15 * X + k_14 Z^2
    hornerPolynomial(fp_tmp[16], p->x, ELLP_Ny_LEN - 3, fp_tmp);     // Horner for the rest
    fp_mul(fp_tmp[16], fp_tmp[16], p->y);                           // Ny * Y
    
    // x = Nx/Dx
    // compute Dx
    compute_map_zvals(iso->xd, fp_tmp + 22, deg_dx, fp_tmp);         // k_(10-i) Z^(2i)
    fp_add(fp_tmp[14], p->x, fp_tmp[deg_dx - 1]);  // X + k_9 Z^2 
    hornerPolynomial(fp_tmp[14], p->x, deg_dx - 2, fp_tmp);    // Horner for the rest
    fp_mul(fp_tmp[14], fp_tmp[14], fp_tmp[31]);                    // Dx * Z^2

    // compute Nx
    compute_map_zvals(bls_prec->iso_Nx, fp_tmp + 21, ELLP_Nx_LEN - 1, fp_tmp);      // k_(11-i) Z^(2i)
    fp_mul(fp_tmp[13], p->x, bls_prec->iso_Nx[ELLP_Nx_LEN - 1]);   // k_11 * X
    fp_add(fp_tmp[13], fp_tmp[13], fp_tmp[ELLP_Nx_LEN - 2]);  // k_11 * X + k_10 * Z^2
    hornerPolynomial(fp_tmp[13], p->x, ELLP_Nx_LEN - 3, fp_tmp);      // Dy: Horner for the rest

    // compute the resulting point (Xo,Yo,Zo)
    fp_mul(r->z, fp_tmp[14], fp_tmp[15]);  // Zo = Dx Dy
    fp_mul(r->x, fp_tmp[13], fp_tmp[15]);  //  Nx Dy
    fp_mul(r->x, r->x, r->z);    // Xo = Nx Dy Z 
    fp_sqr(fp_tmp[12], r->z);                // Zo^2
    fp_mul(r->y, fp_tmp[16], fp_tmp[14]);  // Ny Dx
    fp_mul(r->y, r->y, fp_tmp[12]);   // Yo = Ny Dx Zo^2
    r->coord = JACOB;
}

// map an input point in E to a point in G1 by clearing the cofactor of G1 
static void clear_cofactor(ep_t out, const ep_t in) {
    bn_t z;
    bn_new(z);
    fp_prime_get_par(z);
    // compute 1-z 
    bn_neg(z, z); 
    bn_add_dig(z, z, 1);
    ep_mul_dig(out, in, z->dp[0]); // z fits in 64 bits
    bn_free(z);
}

// construction 2 section 5 in in https://eprint.iacr.org/2019/403.pdf
// evaluate the optimized SSWU map twice, add resulting points, apply isogeny map, clear cofactor
// the result is stored in p
// msg is the input message to hash, must be at least 2*(FP_BYTES+16) = 128 bytes
static void map_to_G1_local(ep_t p, const uint8_t *msg, int len) {
    RLC_TRY {
        if (len < 2*(Fp_BYTES+16)) {
            RLC_THROW(ERR_NO_BUFFER);
        }

        fp_t t1, t2;
        bn_t tmp;
        bn_new(tmp);
        bn_read_bin(tmp, msg, len/2);
        fp_prime_conv(t1, tmp);
        bn_read_bin(tmp, msg + len/2, len - len/2);
        fp_prime_conv(t2, tmp);
        bn_free(tmp);

        ep_t p_temp;
        ep_new(p_temp);
        // first mapping
        map_to_E1_osswu(p_temp, t1); // map to E1
        eval_iso11(p_temp, p_temp); // map to E

        // second mapping
        map_to_E1_osswu(p, t2); // map to E1
        eval_iso11(p, p); // map to E
        // sum 
        // TODO: implement point addition in E1 and apply the isogeny map only once.
        // Gives 4% improvement for map-to-curve overall
        ep_add_jacob(p, p, p_temp);
        
        // clear the cofactor
        clear_cofactor(p, p); // map to G1
        ep_free(p_temp);
    }
    RLC_CATCH_ANY {
		RLC_THROW(ERR_CAUGHT);
	}
}
#endif

// computes a hash of input data to G1
// construction 2 from section 5 in https://eprint.iacr.org/2019/403.pdf
void map_to_G1(ep_t h, const byte* data, const int len) {
    #if hashToPoint==LOCAL_SSWU
    map_to_G1_local(h, data, len);
    #elif hashToPoint==RELIC_SSWU
    ep_map_from_field(h, data, len);
    #endif
}
