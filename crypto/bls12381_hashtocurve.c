// +build relic

#include "bls12381_utils.h"
#include "bls_include.h"

extern prec_st* bls_prec;

#if (hashToPoint==OPSWU)

const uint64_t p_3div4_data[Fp_DIGITS] = {
    0xEE7FBFFFFFFFEAAA, 0x07AAFFFFAC54FFFF, 0xD9CC34A83DAC3D89,
    0xD91DD2E13CE144AF, 0x92C6E9ED90D2EB35, 0x0680447A8E5FF9A6,
};

// (p-1)/2 converted to Montgomery form
const uint64_t fp_p_1div2_data[Fp_DIGITS] = {
    0xa1fafffffffe5557, 0x995bfff976a3fffe, 0x03f41d24d174ceb4,
    0xf6547998c1995dbd, 0x778a468f507a6034, 0x020559931f7f8103,
};

// Isogeny map constants taken from https://eprint.iacr.org/2019/403.pdf (section 4.3)
// and converted to the Mongtomery domain.
const uint64_t a1_data[Fp_DIGITS] = { 
    0x2f65aa0e9af5aa51, 0x86464c2d1e8416c3, 0xb85ce591b7bd31e2,
    0x27e11c91b5f24e7c, 0x28376eda6bfc1835, 0x155455c3e5071d85,
};


const uint64_t b1_data[Fp_DIGITS] = {  
    0xfb996971fe22a1e0, 0x9aa93eb35b742d6f, 0x8c476013de99c5c4,
    0x873e27c3a221e571, 0xca72b5e45a52d888, 0x06824061418a386b,
};

// check if (U/V) is a square, return 1 if yes, 0 otherwise 
// if 1 is returned, out contains sqrt(U/V)
// out should not be the same as U, or V
static int quotient_sqrt(fp_t out, const fp_t u, const fp_t v) {
    fp_st tmp;
    fp_new(&tmp);

    fp_sqr(out, v);                  // V^2
    fp_mul(tmp, u, v);               // UV
    fp_mul(out, out, tmp);           // UV^3
    fp_exp(out, out, &bls_prec->p_3div4);        // (UV^3)^((p-3)/4)
    fp_mul(out, out, tmp);           // UV(UV^3)^((p-3)/4)

    fp_sqr(tmp, out);     // out^2
    fp_mul(tmp, tmp, v);  // out^2 * V
    fp_sub(tmp, tmp, u);   // out^2 * V - U

    int res = fp_cmp_dig(tmp, 0)==RLC_EQ;
    fp_free(&tmp);
    return res;
}


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

const uint64_t iso_Dx_data[ELLP_Dx_LEN][Fp_DIGITS] = {
    {0xb962a077fdb0f945, 0xa6a9740fefda13a0, 0xc14d568c3ed6c544,
     0xb43fc37b908b133e, 0x9c0b3ac929599016, 0x0165aa6c93ad115f,},
    {0x23279a3ba506c1d9, 0x92cfca0a9465176a, 0x3b294ab13755f0ff,
     0x116dda1c5070ae93, 0xed4530924cec2045, 0x083383d6ed81f1ce,},
    {0x9885c2a6449fecfc, 0x4a2b54ccd37733f0, 0x17da9ffd8738c142,
     0xa0fba72732b3fafd, 0xff364f36e54b6812, 0x0f29c13c660523e2,},
    {0xe349cc118278f041, 0xd487228f2f3204fb, 0xc9d325849ade5150,
     0x43a92bd69c15c2df, 0x1c2c7844bc417be4, 0x12025184f407440c,},
    {0x587f65ae6acb057b, 0x1444ef325140201f, 0xfbf995e71270da49,
     0xccda066072436a42, 0x7408904f0f186bb2, 0x13b93c63edf6c015,},
    {0xfb918622cd141920, 0x4a4c64423ecaddb4, 0x0beb232927f7fb26,
     0x30f94df6f83a3dc2, 0xaeedd424d780f388, 0x06cc402dd594bbeb,},
    {0xd41f761151b23f8f, 0x32a92465435719b3, 0x64f436e888c62cb9,
     0xdf70a9a1f757c6e4, 0x6933a38d5b594c81, 0x0c6f7f7237b46606,},
    {0x693c08747876c8f7, 0x22c9850bf9cf80f0, 0x8e9071dab950c124,
     0x89bc62d61c7baf23, 0xbc6be2d8dad57c23, 0x17916987aa14a122,},
    {0x1be3ff439c1316fd, 0x9965243a7571dfa7, 0xc7f7f62962f5cd81,
     0x32c6aa9af394361c, 0xbbc2ee18e1c227f4, 0x0c102cbac531bb34,},
    {0x997614c97bacbf07, 0x61f86372b99192c0, 0x5b8c95fc14353fc3,
     0xca2b066c2a87492f, 0x16178f5bbf698711, 0x12a6dcd7f0f4e0e8,},
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

const uint64_t iso_Dy_data[ELLP_Dy_LEN][Fp_DIGITS] = {
    {0xeb6c359d47e52b1c, 0x18ef5f8a10634d60, 0xddfa71a0889d5b7e,
     0x723e71dcc5fc1323, 0x52f45700b70d5c69, 0x0a8b981ee47691f1,},
    {0x616a3c4f5535b9fb, 0x6f5f037395dbd911, 0xf25f4cc5e35c65da,
     0x3e50dffea3c62658, 0x6a33dca523560776, 0x0fadeff77b6bfe3e,},
    {0x2be9b66df470059c, 0x24a2c159a3d36742, 0x115dbe7ad10c2a37,
     0xb6634a652ee5884d, 0x04fe8bb2b8d81af4, 0x01c2a7a256fe9c41,},
    {0xf27bf8ef3b75a386, 0x898b367476c9073f, 0x24482e6b8c2f4e5f,
     0xc8e0bbd6fe110806, 0x59b0c17f7631448a, 0x11037cd58b3dbfbd,},
    {0x31c7912ea267eec6, 0x1dbf6f1c5fcdb700, 0xd30d4fe3ba86fdb1,
     0x3cae528fbee9a2a4, 0xb1cce69b6aa9ad9a, 0x044393bb632d94fb,},
    {0xc66ef6efeeb5c7e8, 0x9824c289dd72bb55, 0x71b1a4d2f119981d,
     0x104fc1aafb0919cc, 0x0e49df01d942a628, 0x096c3a09773272d4,},
    {0x9abc11eb5fadeff4, 0x32dca50a885728f0, 0xfb1fa3721569734c,
     0xc4b76271ea6506b3, 0xd466a75599ce728e, 0x0c81d4645f4cb6ed,},
    {0x4199f10e5b8be45b, 0xda64e495b1e87930, 0xcb353efe9b33e4ff,
     0x9e9efb24aa6424c6, 0xf08d33680a237465, 0x0d3378023e4c7406,},
    {0x7eb4ae92ec74d3a5, 0xc341b4aa9fac3497, 0x5be603899e907687,
     0x03bfd9cca75cbdeb, 0x564c2935a96bfa93, 0x0ef3c33371e2fdb5,},
    {0x7ee91fd449f6ac2e, 0xe5d5bd5cb9357a30, 0x773a8ca5196b1380,
     0xd0fda172174ed023, 0x6cb95e0fa776aead, 0x0d22d5a40cec7cff,},
    {0xf727e09285fd8519, 0xdc9d55a83017897b, 0x7549d8bd057894ae,
     0x178419613d90d8f8, 0xfce95ebdeb5b490a, 0x0467ffaef23fc49e,},
    {0xc1769e6a7c385f1b, 0x79bc930deac01c03, 0x5461c75a23ede3b5,
     0x6e20829e5c230c45, 0x828e0f1e772a53cd, 0x116aefa749127bff,},
    {0x101c10bf2744c10a, 0xbbf18d053a6a3154, 0xa0ecf39ef026f602,
     0xfc009d4996dc5153, 0xb9000209d5bd08d3, 0x189e5fe4470cd73c,},
    {0x7ebd546ca1575ed2, 0xe47d5a981d081b55, 0x57b2b625b6d4ca21,
     0xb0a1ba04228520cc, 0x98738983c2107ff3, 0x13dddbc4799d81d6,},
    {0x09319f2e39834935, 0x039e952cbdb05c21, 0x55ba77a9a2f76493,
     0xfd04e3dfc6086467, 0xfb95832e7d78742e, 0x0ef9c24eccaf5e0e,},
};

// Maps the field element t to a point p in E1(Fp) where E1: y^2 = g(x) = x^3 + a1*x + b1 
// using optimized non-constant-time SWU impl
// p point is in Jacobian coordinates
static inline void map_to_E1_swu(ep_t p, const fp_t t) {
    const int tmp_len = 6;
    fp_t* fp_tmp = (fp_t*) malloc(tmp_len*sizeof(fp_t));
    for (int i=0; i<tmp_len; i++) fp_new(&fp_tmp[i]);

    // compute numerator and denominator of X0(t) = N / D
    fp_sqr(fp_tmp[0], t);                      // t^2
    fp_sqr(fp_tmp[1], fp_tmp[0]);             // t^4
    fp_sub(fp_tmp[1], fp_tmp[0], fp_tmp[1]);  // t^2 - t^4
    fp_set_dig(fp_tmp[3], 1);
    fp_sub(fp_tmp[2], fp_tmp[3], fp_tmp[1]);        // t^4 - t^2 + 1
    fp_mul(fp_tmp[2], fp_tmp[2], bls_prec->b1);     // N = b * (t^4 - t^2 + 1)                
    fp_mul(fp_tmp[1], fp_tmp[1], bls_prec->a1);     // D = a * (t^2 - t^4)                    
    if (fp_cmp_dig(fp_tmp[1], 0) == RLC_EQ) {
        // t was 0, -1, 1, so num is b and den is 0; set den to -a, because -b/a is square in Fp
        fp_neg_basic(fp_tmp[1], bls_prec->a1);
    }

    // compute numerator and denominator of g(X0(t)) = U / V 
    // U = N^3 + a1 * N * D^2 + b1 D^3
    // V = D^3
    fp_sqr(fp_tmp[3], fp_tmp[1]);              // D^2
    fp_mul(fp_tmp[4], fp_tmp[2], fp_tmp[3]);  // N * D^2
    fp_mul(fp_tmp[4], fp_tmp[4], bls_prec->a1);      // a * N * D^2
                                                   
    fp_mul(fp_tmp[3], fp_tmp[3], fp_tmp[1]);  // V = D^3
    fp_mul(fp_tmp[5], fp_tmp[3], bls_prec->b1);      // b1 * D^3
    fp_add(fp_tmp[4], fp_tmp[4], fp_tmp[5]);   // a1 * N * D^2 + b1 * D^3
                                                   
    fp_sqr(fp_tmp[5], fp_tmp[2]);              // N^2
    fp_mul(fp_tmp[5], fp_tmp[5], fp_tmp[2]);  // N^3
    fp_add(fp_tmp[4], fp_tmp[4], fp_tmp[5]);   // U

    // compute sqrt(U/V)
    if (!quotient_sqrt(fp_tmp[5], fp_tmp[4], fp_tmp[3])) {
        // g(X0(t)) was nonsquare, so convert to g(X1(t))
        fp_mul(fp_tmp[5], fp_tmp[5], fp_tmp[0]);  // t^2 * sqrtCand
        fp_mul(fp_tmp[5], fp_tmp[5], t);           // t^3 * sqrtCand
        fp_mul(fp_tmp[2], fp_tmp[2], fp_tmp[0]);  // b * t^2 * (t^4 - t^2 + 1)
        fp_neg_basic(fp_tmp[2], fp_tmp[2]);        // N = - b * t^2 * (t^4 - t^2 + 1)
    } else if (dv_cmp(bls_prec->fp_p_1div2, t, Fp_DIGITS) ==  RLC_LT) {
        // g(X0(t)) was square and t is negative, so negate y
        fp_neg_basic(fp_tmp[5], fp_tmp[5]);  // negate y because t is negative
    }

    // convert (x,y)=(N/D, y) into (X,Y,Z) where Z=D
    // Z = D, X = x*D^2 = N.D , Y = y*D^3
    fp_mul(p->x, fp_tmp[2], fp_tmp[1]);  // X = N*D
    fp_mul(p->y, fp_tmp[5], fp_tmp[3]);  // Y = y*D^3
    fp_copy(p->z, fp_tmp[1]);
    p->coord = JACOB;
    
    for (int i=0; i<tmp_len; i++) fp_free(&fp_tmp[i]);
    free(fp_tmp);
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
    const int tmp_len = 32;
    fp_t* fp_tmp = (fp_t*) malloc(tmp_len*sizeof(fp_t));
    for (int i=0; i<tmp_len; i++) fp_new(&fp_tmp[i]);

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

    // y = Ny/Dy
    // compute Dy
    compute_map_zvals(bls_prec->iso_Dy, fp_tmp + 17, ELLP_Dy_LEN, fp_tmp);     // k_(15-i) Z^(2i)
    fp_add(fp_tmp[16], p->x, fp_tmp[ELLP_Dy_LEN - 1]);        // X + k_14 Z^2 
    hornerPolynomial(fp_tmp[16], p->x, ELLP_Dy_LEN - 2, fp_tmp);    // Horner for the rest
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
    compute_map_zvals(bls_prec->iso_Dx, fp_tmp + 22, ELLP_Dx_LEN, fp_tmp);         // k_(10-i) Z^(2i)
    fp_add(fp_tmp[14], p->x, fp_tmp[ELLP_Dx_LEN - 1]);  // X + k_9 Z^2 
    hornerPolynomial(fp_tmp[14], p->x, ELLP_Dx_LEN - 2, fp_tmp);    // Horner for the rest
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
    
    for (int i=0; i<tmp_len; i++) fp_free(&fp_tmp[i]);
    free(fp_tmp);
}

// map an input point in E to a point in G1 by clearing the cofactor of G1 
static void clear_cofactor(ep_t out, const ep_t in) {
    bn_t z;
    bn_new(z);
    fp_prime_get_par(z);
    // compute 1-z 
    bn_neg(z, z);  // keep -z in only 64 bits
    bn_add_dig(z, z, 1);
    ep_mul_dig(out, in, z[0].dp[0]);
    bn_free(z);
}

// construction 2 section 5 in in https://eprint.iacr.org/2019/403.pdf
// evaluate the optimized SWU map twice, add resulting points, apply isogeny map, clear cofactor
// the result is stored in p
// msg is the input message to hash, must be at least 2*(FP_BYTES+16) = 128 bytes
static void map_to_G1_opswu(ep_t p, const uint8_t *msg, int len) {
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
        map_to_E1_swu(p_temp, t1); // map to E1
        eval_iso11(p_temp, p_temp); // map to E
        // second mapping
        map_to_E1_swu(p, t2); // map to E1
        eval_iso11(p, p); // map to E
        // sum and clear the cofactor
        // TODO: implement point addition in E1 and apply the isogeny map
        // only once.
        ep_add_jacob(p, p, p_temp);
        clear_cofactor(p, p_temp); // map to G1
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
    #if hashToPoint==OPSWU
    // implementation using different mapping parameters than the IRTF draft
    map_to_G1_opswu(h, data, len);
    #elif hashToPoint==RELIC_OPSWU
    // relic implementation compliant the IRTF draft
    ep_map_from_field(h, data, len);
    #endif
}
