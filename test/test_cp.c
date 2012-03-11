/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2011 RELIC Authors
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file
 * for contact information.
 *
 * RELIC is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2.1 of the License, or (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with RELIC. If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * @file
 *
 * Tests for the binary elliptic curve arithmetic module.
 *
 * @version $Id$
 * @ingroup test
 */

#include <stdio.h>

#include "relic.h"
#include "relic_test.h"

static int rsa(void) {
	int code = STS_ERR;
	rsa_t pub, prv;
	unsigned char in[10];
	unsigned char out[BN_BITS / 8 + 1];
	int in_len, out_len;
	int result;

	rsa_null(pub);
	rsa_null(prv);

	TRY {
		rsa_new(pub);
		rsa_new(prv);

		result = cp_rsa_gen(pub, prv, BN_BITS);

		TEST_BEGIN("rsa encryption/decryption is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_enc(out, &out_len, in, in_len, pub) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_dec(out, &out_len, out, out_len, prv) == STS_OK,
					end);
			TEST_ASSERT(memcmp(in, out, out_len) == 0, end);
		} TEST_END;

#if CP_RSA == BASIC || !defined(STRIP)
		result = cp_rsa_gen_basic(pub, prv, BN_BITS);

		TEST_BEGIN("basic rsa encryption/decryption is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_enc(out, &out_len, in, in_len, pub) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_dec_basic(out, &out_len, out, out_len,
							prv) == STS_OK, end);
			TEST_ASSERT(memcmp(in, out, out_len) == 0, end);
		} TEST_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
		result = cp_rsa_gen_quick(pub, prv, BN_BITS);

		TEST_BEGIN("fast rsa encryption/decryption is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_enc(out, &out_len, in, in_len, pub) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_dec_quick(out, &out_len, out, out_len,
							prv) == STS_OK, end);
			TEST_ASSERT(memcmp(in, out, out_len) == 0, end);
		} TEST_END;
#endif

		result = cp_rsa_gen(pub, prv, BN_BITS);

		TEST_BEGIN("rsa signature/verification is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_sign(out, &out_len, in, in_len, prv) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_ver(out, out_len, in, in_len, pub) == 1, end);
		} TEST_END;

#if CP_RSA == BASIC || !defined(STRIP)
		result = cp_rsa_gen_basic(pub, prv, BN_BITS);

		TEST_BEGIN("basic rsa signature/verification is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_sign_basic(out, &out_len, in, in_len,
							prv) == STS_OK, end);
			TEST_ASSERT(cp_rsa_ver(out, out_len, in, in_len, pub) == 1, end);
		} TEST_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
		result = cp_rsa_gen_quick(pub, prv, BN_BITS);

		TEST_BEGIN("fast rsa signature/verification is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_sign_quick(out, &out_len, in, in_len,
							prv) == STS_OK, end);
			TEST_ASSERT(cp_rsa_ver(out, out_len, in, in_len, pub) == 1, end);
		} TEST_END;
#endif

	} CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	rsa_free(pub);
	rsa_free(prv);
	return code;
}

static int rabin(void) {
	int code = STS_ERR;
	rabin_t pub, prv;
	unsigned char in[10];
	unsigned char out[BN_BITS / 8 + 1];
	int in_len, out_len;
	int result;

	rabin_null(pub);
	rabin_null(prv);

	TRY {
		rabin_new(pub);
		rabin_new(prv);

		result = cp_rabin_gen(pub, prv, BN_BITS);

		TEST_BEGIN("rabin encryption/decryption is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rabin_enc(out, &out_len, in, in_len, pub) == STS_OK,
					end);
			TEST_ASSERT(cp_rabin_dec(out, &out_len, out, out_len,
							prv) == STS_OK, end);
			TEST_ASSERT(memcmp(in, out, out_len) == 0, end);
		} TEST_END;
	} CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	rabin_free(pub);
	rabin_free(prv);
	return code;
}

#if defined(WITH_EC)

#if defined(EP_ORDIN) && FP_PRIME == 160

#define SECG_P160_A		"AA374FFC3CE144E6B073307972CB6D57B2A4E982"
#define SECG_P160_B		"45FB58A92A17AD4B15101C66E74F277E2B460866"
#define SECG_P160_A_X	"51B4496FECC406ED0E75A24A3C03206251419DC0"
#define SECG_P160_A_Y	"C28DCB4B73A514B468D793894F381CCC1756AA6C"
#define SECG_P160_B_X	"49B41E0E9C0369C2328739D90F63D56707C6E5BC"
#define SECG_P160_B_Y	"26E008B567015ED96D232A03111C3EDC0E9C8F83"

unsigned char resultp[] = {
	0x74, 0x4A, 0xB7, 0x03, 0xF5, 0xBC, 0x08, 0x2E, 0x59, 0x18, 0x5F, 0x6D,
	0x04, 0x9D, 0x2D, 0x36, 0x7D, 0xB2, 0x45, 0xC2
};

#endif

#if defined(EB_KBLTZ) && FB_POLYN == 163

#define NIST_K163_A		"3A41434AA99C2EF40C8495B2ED9739CB2155A1E0D"
#define NIST_K163_B		"057E8A78E842BF4ACD5C315AA0569DB1703541D96"
#define NIST_K163_A_X	"37D529FA37E42195F10111127FFB2BB38644806BC"
#define NIST_K163_A_Y	"447026EEE8B34157F3EB51BE5185D2BE0249ED776"
#define NIST_K163_B_X	"72783FAAB9549002B4F13140B88132D1C75B3886C"
#define NIST_K163_B_Y	"5A976794EA79A4DE26E2E19418F097942C08641C7"

unsigned char resultk[] = {
	0x66, 0x55, 0xA9, 0xC8, 0xF9, 0xE5, 0x93, 0x14, 0x9D, 0xB2, 0x4C, 0x91,
	0xCE, 0x62, 0x16, 0x41, 0x03, 0x5C, 0x92, 0x82
};

#endif

#define ASSIGNP(CURVE)														\
	FETCH(str, CURVE##_A, sizeof(CURVE##_A));								\
	bn_read_str(d_a, str, strlen(str), 16);									\
	FETCH(str, CURVE##_A_X, sizeof(CURVE##_A_X));							\
	fp_read(q_a->x, str, strlen(str), 16);									\
	FETCH(str, CURVE##_A_Y, sizeof(CURVE##_A_Y));							\
	fp_read(q_a->y, str, strlen(str), 16);									\
	fp_set_dig(q_b->z, 1);													\
	FETCH(str, CURVE##_B, sizeof(CURVE##_B));								\
	bn_read_str(d_b, str, strlen(str), 16);									\
	FETCH(str, CURVE##_B_X, sizeof(CURVE##_B_X));							\
	fp_read(q_b->x, str, strlen(str), 16);									\
	FETCH(str, CURVE##_B_Y, sizeof(CURVE##_B_Y));							\
	fp_read(q_b->y, str, strlen(str), 16);									\
	fp_set_dig(q_b->z, 1);

#define ASSIGNK(CURVE)														\
	FETCH(str, CURVE##_A, sizeof(CURVE##_A));								\
	bn_read_str(d_a, str, strlen(str), 16);									\
	FETCH(str, CURVE##_A_X, sizeof(CURVE##_A_X));							\
	fb_read(q_a->x, str, strlen(str), 16);									\
	FETCH(str, CURVE##_A_Y, sizeof(CURVE##_A_Y));							\
	fb_read(q_a->y, str, strlen(str), 16);									\
	fb_set_dig(q_b->z, 1);													\
	FETCH(str, CURVE##_B, sizeof(CURVE##_B));								\
	bn_read_str(d_b, str, strlen(str), 16);									\
	FETCH(str, CURVE##_B_X, sizeof(CURVE##_B_X));							\
	fb_read(q_b->x, str, strlen(str), 16);									\
	FETCH(str, CURVE##_B_Y, sizeof(CURVE##_B_Y));							\
	fb_read(q_b->y, str, strlen(str), 16);									\
	fb_set_dig(q_b->z, 1);

static int ecdh(void) {
	int code = STS_ERR;
	char str[2 * EC_BYTES + 1];
	bn_t d_a, d_b;
	ec_t q_a, q_b;
	unsigned char key[MD_LEN], key1[MD_LEN], key2[MD_LEN];

	bn_null(d_a);
	bn_null(d_b);
	ec_null(q_a);
	ec_null(q_b);

	TRY {
		bn_new(d_a);
		bn_new(d_b);
		ec_new(q_a);
		ec_new(q_b);

		TEST_BEGIN("ecdh is correct") {
			cp_ecdh_gen(d_a, q_a);
			cp_ecdh_gen(d_b, q_b);
			cp_ecdh_key(key1, MD_LEN, d_b, q_a);
			cp_ecdh_key(key2, MD_LEN, d_a, q_b);
			TEST_ASSERT(memcmp(key1, key2, MD_LEN) == 0, end);
		} TEST_END;

#if MD_MAP == SHONE
		TEST_ONCE("ecdh satisfies test vectors") {
			switch (ec_param_get()) {
#if EC_CUR == PRIME

#if defined(EP_ORDIN) && FP_PRIME == 160
				case SECG_P160:
					ASSIGNP(SECG_P160);
					memcpy(key, resultp, MD_LEN);
					break;
#endif

#else /* EC_CUR == CHAR2 */

#if defined(EB_KBLTZ) && FB_POLYN == 163
				case NIST_K163:
					ASSIGNK(NIST_K163);
					memcpy(key, resultk, MD_LEN);
					break;
#endif

#endif
				default:
					cp_ecdh_gen(d_a, q_a);
					cp_ecdh_gen(d_b, q_b);
					cp_ecdh_key(key1, MD_LEN, d_b, q_a);
					cp_ecdh_key(key2, MD_LEN, d_a, q_b);
					memcpy(key, key1, MD_LEN);
					TEST_ASSERT(memcmp(key1, key2, MD_LEN) == 0, end);
					break;
			}
			TEST_ASSERT(ec_is_valid(q_a) == 1, end);
			TEST_ASSERT(ec_is_valid(q_b) == 1, end);
			cp_ecdh_key(key1, MD_LEN, d_b, q_a);
			cp_ecdh_key(key2, MD_LEN, d_b, q_a);
			TEST_ASSERT(memcmp(key1, key, MD_LEN) == 0, end);
			TEST_ASSERT(memcmp(key2, key, MD_LEN) == 0, end);
		}
		TEST_END;
#endif
		(void)str;
		(void)key;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	bn_free(d_a);
	bn_free(d_b);
	ec_free(q_a);
	ec_free(q_b);
	return code;
}

static int ecmqv(void) {
	int code = STS_ERR;
	bn_t d1_a, d1_b;
	bn_t d2_a, d2_b;
	ec_t q1_a, q1_b;
	ec_t q2_a, q2_b;
	unsigned char key1[MD_LEN], key2[MD_LEN];

	bn_null(d1_a);
	bn_null(d1_b);
	ec_null(q1_a);
	ec_null(q1_b);
	bn_null(d2_a);
	bn_null(d2_b);
	ec_null(q2_a);
	ec_null(q2_b);

	TRY {
		bn_new(d1_a);
		bn_new(d1_b);
		ec_new(q1_a);
		ec_new(q1_b);
		bn_new(d2_a);
		bn_new(d2_b);
		ec_new(q2_a);
		ec_new(q2_b);

		TEST_BEGIN("ecmqv is correct") {
			cp_ecmqv_gen(d1_a, q1_a);
			cp_ecmqv_gen(d2_a, q2_a);
			cp_ecmqv_gen(d1_b, q1_b);
			cp_ecmqv_gen(d2_b, q2_b);
			cp_ecmqv_key(key1, MD_LEN, d1_b, d2_b, q2_b, q1_a, q2_a);
			cp_ecmqv_key(key2, MD_LEN, d1_a, d2_a, q2_a, q1_b, q2_b);
			TEST_ASSERT(memcmp(key1, key2, MD_LEN) == 0, end);
		} TEST_END;
	}
	CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	bn_free(d1_a);
	bn_free(d1_b);
	ec_free(q1_a);
	ec_free(q1_b);
	bn_free(d2_a);
	bn_free(d2_b);
	ec_free(q2_a);
	ec_free(q2_b);
	return code;
}

static int ecdsa(void) {
	int code = STS_ERR;
	bn_t d, r;
	ec_t q;
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };

	bn_null(d);
	bn_null(r);
	ec_null(q);

	TRY {
		bn_new(d);
		bn_new(r);
		ec_new(q);

		TEST_BEGIN("ecdsa is correct") {
			cp_ecdsa_gen(d, q);
			cp_ecdsa_sign(r, d, msg, 5, d);
			TEST_ASSERT(cp_ecdsa_ver(r, d, msg, 5, q) == 1, end);
		} TEST_END;
	} CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	bn_free(d);
	bn_free(r);
	ec_free(q);
	return code;
}

static int ecss(void) {
	int code = STS_ERR;
	bn_t d, r;
	ec_t q;
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };

	bn_null(d);
	bn_null(r);
	ec_null(q);

	TRY {
		bn_new(d);
		bn_new(r);
		ec_new(q);

		TEST_BEGIN("ecss is correct") {
			cp_ecss_gen(d, q);
			cp_ecss_sign(r, d, msg, 5, d);
			TEST_ASSERT(cp_ecss_ver(r, d, msg, 5, q) == 1, end);
		} TEST_END;
	} CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	bn_free(d);
	bn_free(r);
	ec_free(q);
	return code;
}

#endif

#if defined(WITH_PC)

static int sokaka(void) {
	int code = STS_ERR;
	sokaka_t s_i;
	bn_t s;
	unsigned char key1[MD_LEN], key2[MD_LEN];
	char id_a[5] = { 'A', 'l', 'i', 'c', 'e' };
	char id_b[3] = { 'B', 'o', 'b' };

	sokaka_null(s_i);

	TRY {
		sokaka_new(s_i);
		bn_new(s);

		cp_sokaka_gen(s);

		TEST_BEGIN
				("sakai-ohgishi-kasahara authenticated key agreement is correct")
		{
			cp_sokaka_gen_prv(s_i, id_a, 5, s);
			cp_sokaka_key(key1, MD_LEN, id_a, 5, s_i, id_b, 3);
			cp_sokaka_gen_prv(s_i, id_b, 3, s);
			cp_sokaka_key(key2, MD_LEN, id_b, 3, s_i, id_a, 5);
			TEST_ASSERT(memcmp(key1, key2, MD_LEN) == 0, end);
		} TEST_END;

	} CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	sokaka_free(s_i);
	return code;
}

static int bls(void) {
	int code = STS_ERR;
	bn_t d;
	g1_t s;
	g2_t q;
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };

	bn_null(d);
	g1_null(s);
	g2_null(q);

	TRY {
		bn_new(d);
		g1_new(s);
		g2_new(q);

		TEST_BEGIN("boneh-lynn-schacham short signature is correct") {
			cp_bls_gen(d, q);
			cp_bls_sign(s, msg, 5, d);
			TEST_ASSERT(cp_bls_ver(s, msg, 5, q) == 1, end);
		} TEST_END;
	} CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	bn_free(d);
	g1_free(s);
	g2_free(q);
	return code;
}

static int bbs(void) {
	int code = STS_ERR;
	int b;
	bn_t d;
	g1_t s;
	g2_t q;
	gt_t z;
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };

	bn_null(d);
	g1_null(s);
	g2_null(q);
	gt_null(z);

	TRY {
		bn_new(d);
		g1_new(s);
		g2_new(q);
		gt_new(z);

		TEST_BEGIN("boneh-boyen short signature is correct") {
			cp_bbs_gen(d, q, z);
			cp_bbs_sign(&b, s, msg, 5, d);
			TEST_ASSERT(cp_bbs_ver(b, s, msg, 5, q, z) == 1, end);
		} TEST_END;
	} CATCH_ANY {
		ERROR(end);
	}
	code = STS_OK;

  end:
	bn_free(d);
	g1_free(s);
	g2_free(q);
	return code;
}

#endif

int main(void) {
	core_init();

	util_print_banner("Tests for the CP module", 0);

#if defined(WITH_BN)
	util_print_banner("Protocols based on prime factorization:\n", 0);

	if (rsa() != STS_OK) {
		core_clean();
		return 1;
	}

	if (rabin() != STS_OK) {
		core_clean();
		return 1;
	}
#endif

#if defined(WITH_EC)
	util_print_banner("Protocols based on elliptic curves:\n", 0);
	if (ec_param_set_any() == STS_OK) {
		if (ecdh() != STS_OK) {
			core_clean();
			return 1;
		}
		if (ecmqv() != STS_OK) {
			core_clean();
			return 1;
		}
		if (ecdsa() != STS_OK) {
			core_clean();
			return 1;
		}
		if (ecss() != STS_OK) {
			core_clean();
			return 1;
		}
	} else {
		THROW(ERR_NO_CURVE);
	}
#endif

#if defined(WITH_PC)
	util_print_banner("Protocols based on pairings:\n", 0);
	if (pc_param_set_any() == STS_OK) {
		if (sokaka() != STS_OK) {
			core_clean();
			return 1;
		}
		if (bls() != STS_OK) {
			core_clean();
			return 1;
		}
		if (bbs() != STS_OK) {
			core_clean();
			return 1;
		}
	} else {
		THROW(ERR_NO_CURVE);
	}
#endif

	core_clean();
	return 0;
}
