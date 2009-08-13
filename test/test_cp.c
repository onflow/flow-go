/*
 * Copyright 2007-2009 RELIC Project
 *
 * This file is part of RELIC. RELIC is legal property of its developers,
 * whose names are not listed here. Please refer to the COPYRIGHT file.
 *
 * RELIC is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * RELIC is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
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
#include<string.h>
#include<math.h>
#include<stdlib.h>
#include<stdint.h>

#include "relic.h"
#include "relic_test.h"

int rsa(void) {
	err_t e;
	int code = STS_ERR;
	rsa_pub_t pub;
	rsa_prv_t prv;
	unsigned char in[10];
	unsigned char out[BN_BITS / 8 + 1];
	int in_len, out_len;
	int result;

	TRY {
		bn_new(pub.e);
		bn_new(pub.n);
		bn_new(prv.d);
		bn_new(prv.dp);
		bn_new(prv.dq);
		bn_new(prv.p);
		bn_new(prv.q);
		bn_new(prv.qi);
		bn_new(prv.n);

		result = cp_rsa_gen(&pub, &prv, BN_BITS);

		TEST_BEGIN("rsa encryption/decryption is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_enc(out, &out_len, in, in_len, &pub) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_dec(out, &out_len, out, out_len, &prv) == STS_OK,
					end);
			TEST_ASSERT(memcmp(in, out, out_len) == 0, end);
		} TEST_END;

#if CP_RSA == BASIC || !defined(STRIP)
		result = cp_rsa_gen_basic(&pub, &prv, BN_BITS);

		TEST_BEGIN("basic rsa encryption/decryption is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_enc(out, &out_len, in, in_len, &pub) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_dec_basic(out, &out_len, out, out_len, &prv) == STS_OK,
					end);
			TEST_ASSERT(memcmp(in, out, out_len) == 0, end);
		} TEST_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
		result = cp_rsa_gen_quick(&pub, &prv, BN_BITS);

		TEST_BEGIN("fast rsa encryption/decryption is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_enc(out, &out_len, in, in_len, &pub) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_dec_quick(out, &out_len, out, out_len, &prv) == STS_OK,
					end);
			TEST_ASSERT(memcmp(in, out, out_len) == 0, end);
		} TEST_END;
#endif

		result = cp_rsa_gen(&pub, &prv, BN_BITS);

		TEST_BEGIN("rsa signature/verification is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_sign(out, &out_len, in, in_len, &prv) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_ver(out, out_len, in, in_len, &pub) == 1,
					end);
		} TEST_END;

#if CP_RSA == BASIC || !defined(STRIP)
		result = cp_rsa_gen_basic(&pub, &prv, BN_BITS);

		TEST_BEGIN("basic rsa signature/verification is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_sign_basic(out, &out_len, in, in_len, &prv) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_ver(out, out_len, in, in_len, &pub) == 1,
					end);
		} TEST_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
		result = cp_rsa_gen_quick(&pub, &prv, BN_BITS);

		TEST_BEGIN("fast rsa signature/verification is correct") {
			TEST_ASSERT(result == STS_OK, end);
			in_len = 10;
			out_len = BN_BITS / 8 + 1;
			rand_bytes(in, in_len);
			TEST_ASSERT(cp_rsa_sign_quick(out, &out_len, in, in_len, &prv) == STS_OK,
					end);
			TEST_ASSERT(cp_rsa_ver(out, out_len, in, in_len, &pub) == 1,
					end);
		} TEST_END;
#endif

	} CATCH(e) {
		switch (e) {
			case ERR_NO_MEMORY:
				util_print("FATAL ERROR!\n");
				ERROR(end);
				break;
		}
	}
	code = STS_OK;

  end:
	return code;
}

#if defined(WITH_EB)

int ecdsa(void) {
	err_t e;
	int code = STS_ERR;
	bn_t d = NULL, r = NULL;
	eb_t q = NULL;
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };

	TRY {
		bn_new(d);
		bn_new(r);
		eb_new(q);

		TEST_BEGIN("ecdsa is correct") {
			cp_ecdsa_gen(d, q);
			cp_ecdsa_sign(r, d, msg, 5, d);
			TEST_ASSERT(cp_ecdsa_ver(r, d, msg, 5, q) == 1, end);
		} TEST_END;

#if CP_ECDSA == BASIC || !defined(STRIP)
		TEST_BEGIN("basic ecdsa is correct") {
			cp_ecdsa_gen(d, q);
			cp_ecdsa_sign_basic(r, d, msg, 5, d);
			TEST_ASSERT(cp_ecdsa_ver_basic(r, d, msg, 5, q) == 1, end);
		} TEST_END;
#endif

	} CATCH(e) {
		switch (e) {
			case ERR_NO_MEMORY:
				util_print("FATAL ERROR!\n");
				ERROR(end);
				break;
		}
	}
	code = STS_OK;

  end:
	bn_free(d);
	bn_free(r);
	eb_free(q);
	return code;
}

#endif

#if defined(WITH_PB)

int sokaka(void) {
	err_t e;
	int code = STS_ERR;
	bn_t s;
	eb_t p_a = NULL, p_b = NULL, s_a = NULL, s_b = NULL;
	fb4_t key1, key2;

	TRY {
		bn_new(s);
		eb_new(p_a);
		eb_new(p_b);
		eb_new(s_a);
		eb_new(s_b);
		fb4_new(key1);
		fb4_new(key2);

		cp_sokaka_gen(s);

		TEST_BEGIN("sakai-ohgishi-kasahara authenticated key agreement is correct") {
			cp_sokaka_gen_pub(p_a, "Alice", strlen("Alice"));
			cp_sokaka_gen_pub(p_b, "Bob", strlen("Bob"));
			cp_sokaka_gen_prv(s_a, "Alice", strlen("Alice"), s);
			cp_sokaka_gen_prv(s_b, "Bob", strlen("Bob"), s);
			cp_sokaka_key(key1, p_b, s_a);
			cp_sokaka_key(key2, p_a, s_b);
			TEST_ASSERT(fb4_cmp(key1, key2) == CMP_EQ, end);
		} TEST_END;

	} CATCH(e) {
		switch (e) {
			case ERR_NO_MEMORY:
				util_print("FATAL ERROR!\n");
				ERROR(end);
				break;
		}
	}
	code = STS_OK;

  end:
	eb_free(p_a);
	eb_free(p_b);
	eb_free(s_a);
	eb_free(s_b);
	fb4_free(key1);
	fb4_free(key2);
	return code;
}

#endif

int main(void) {
	int r0, r1;
	core_init();

	util_print_label("Tests for the CP module", 0);

	util_print_label("Protocols based on prime factorization:", 1);

#if defined(WITH_BN)
	if (rsa() != STS_OK) {
		core_clean();
		return 1;
	}
#endif

	util_print_label("Protocols based on elliptic curves:", 1);

#if defined(WITH_EB)
#if defined(EB_STAND) && defined(EB_ORDIN)
	r0 = eb_param_set_any_ordin();
	if (r0 == STS_OK) {
		eb_param_print();
		if (ecdsa() != STS_OK) {
			core_clean();
			return 1;
		}
	}
#endif

#if defined(EB_STAND) && defined(EB_KBLTZ)
	r1 = eb_param_set_any_ordin();
	if (r1 == STS_OK) {
		eb_param_print();
		if (ecdsa() != STS_OK) {
			core_clean();
			return 1;
		}
	}
#endif

	if (r0 == STS_ERR && r1 == STS_ERR) {
		THROW(ERR_NO_CURVE);
	}
#endif
	core_clean();
	return 0;
}
