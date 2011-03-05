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
 * Benchmarks for the cryptographic protocols.
 *
 * @version $Id$
 * @ingroup bench
 */

#include <stdio.h>

#include "relic.h"
#include "relic_bench.h"

#if defined(WITH_BN)

static void rsa(void) {
	rsa_t pub, prv;
	unsigned char in[1000], new[1000];
	unsigned char out[BN_BITS / 8 + 1];
	int in_len, out_len, new_len;

	rsa_null(pub);
	rsa_null(prv);

	rsa_new(pub);
	rsa_new(prv);

	BENCH_ONCE("cp_rsa_gen", cp_rsa_gen(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_enc") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_enc(out, &out_len, in, in_len, pub));
		cp_rsa_dec(new, &new_len, out, out_len, prv);
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_dec") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_enc(out, &out_len, in, in_len, pub);
		BENCH_ADD(cp_rsa_dec(new, &new_len, out, out_len, prv));
	} BENCH_END;

#if CP_RSA == BASIC || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_basic", cp_rsa_gen_basic(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_dec_basic") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_enc(out, &out_len, in, in_len, pub);
		BENCH_ADD(cp_rsa_dec_basic(new, &new_len, out, out_len, prv));
	} BENCH_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_quick", cp_rsa_gen_quick(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_dec_quick") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_enc(out, &out_len, in, in_len, pub);
		BENCH_ADD(cp_rsa_dec_quick(new, &new_len, out, out_len, prv));
	} BENCH_END;
#endif

	BENCH_ONCE("cp_rsa_gen", cp_rsa_gen(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sign") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_sign(out, &out_len, in, in_len, prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_ver") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_sign(out, &out_len, in, in_len, prv);
		BENCH_ADD(cp_rsa_ver(out, out_len, in, in_len, pub));
	} BENCH_END;

#if CP_RSA == BASIC || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_basic", cp_rsa_gen_basic(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sign_basic") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_sign_basic(out, &out_len, in, in_len, prv));
	} BENCH_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_quick", cp_rsa_gen_quick(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sign_quick") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_sign_quick(out, &out_len, in, in_len, prv));
	} BENCH_END;
#endif

	rsa_free(pub);
	rsa_free(prv);
}

static void rabin(void) {
	rabin_t pub, prv;
	unsigned char in[1000], new[1000];
	unsigned char out[BN_BITS / 8 + 1];
	int in_len, out_len, new_len;

	rabin_null(pub);
	rabin_null(prv);

	rabin_new(pub);
	rabin_new(prv);

	BENCH_ONCE("cp_rabin_gen", cp_rabin_gen(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rabin_enc") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 9;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rabin_enc(out, &out_len, in, in_len, pub));
		cp_rabin_dec(new, &new_len, out, out_len, prv);
	} BENCH_END;

	BENCH_BEGIN("cp_rabin_dec") {
		bn_size_bin(&in_len, pub->n);
		in_len -= 9;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rabin_enc(out, &out_len, in, in_len, pub);
		BENCH_ADD(cp_rabin_dec(new, &new_len, out, out_len, prv));
	} BENCH_END;

	rabin_free(pub);
	rabin_free(prv);
}

#endif

#if defined(WITH_EC)

static void ecdh(void) {
	bn_t d;
	ec_t p;
	unsigned char key[MD_LEN];

	bn_null(d);
	ec_null(p);

	bn_new(d);
	ec_new(p);

	BENCH_BEGIN("cp_ecdh_gen") {
		BENCH_ADD(cp_ecdh_gen(d, p));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdh_key") {
		BENCH_ADD(cp_ecdh_key(key, MD_LEN, d, p));
	}
	BENCH_END;

	bn_free(d);
	ec_free(p);
}

static void ecdsa(void) {
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };
	bn_t r, s, d;
	ec_t p;

	bn_null(r);
	bn_null(s);
	bn_null(d);
	ec_null(p);

	bn_new(r);
	bn_new(s);
	bn_new(d);
	ec_new(p);

	BENCH_BEGIN("cp_ecdsa_gen") {
		BENCH_ADD(cp_ecdsa_gen(d, p));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdsa_sign") {
		BENCH_ADD(cp_ecdsa_sign(r, s, msg, 5, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdsa_ver") {
		BENCH_ADD(cp_ecdsa_ver(r, s, msg, 5, p));
	}
	BENCH_END;

	bn_free(r);
	bn_free(s);
	bn_free(d);
	ec_free(p);
}

#endif

#if defined(WITH_PC)

static void sokaka(void) {
	sokaka_t s_a;
	bn_t s;
	unsigned char key1[MD_LEN];
	char id_a[5] = {'A', 'l', 'i', 'c', 'e'};
	char id_b[3] = {'B', 'o', 'b'};

	sokaka_null(s_a);

	sokaka_new(s_a);
	bn_new(s);

	cp_sokaka_gen(s);

	BENCH_BEGIN("cp_sokaka_gen") {
		BENCH_ADD(cp_sokaka_gen(s));
	} BENCH_END;

	BENCH_BEGIN("cp_sokaka_gen_prv") {
		BENCH_ADD(cp_sokaka_gen_prv(s_a, id_a, 5, s));
	} BENCH_END;

	BENCH_BEGIN("cp_sokaka_key") {
		BENCH_ADD(cp_sokaka_key(key1, MD_LEN, id_a, 5, s_a, id_b, 3));
	} BENCH_END;

	sokaka_free(s_a);
	bn_free(s);
}

static void bls(void) {
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };
	g1_t s;
	g2_t p;
	bn_t d;

	g1_null(s);
	g2_null(p);
	bn_null(d);

	g1_new(s);
	g2_new(p);
	bn_new(d);

	BENCH_BEGIN("cp_bls_gen") {
		BENCH_ADD(cp_bls_gen(d, p));
	}
	BENCH_END;

	BENCH_BEGIN("cp_bls_sign") {
		BENCH_ADD(cp_bls_sign(s, msg, 5, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_bls_ver") {
		BENCH_ADD(cp_bls_ver(s, msg, 5, p));
	}
	BENCH_END;

	g1_free(s);
	bn_free(d);
	g2_free(p);
}

static void bbs(void) {
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };
	int b;
	g1_t s;
	g2_t p;
	gt_t z;
	bn_t d;

	g1_null(s);
	g2_null(p);
	gt_null(z);
	bn_null(d);

	g1_new(s);
	g2_new(p);
	gt_new(z);
	bn_new(d);

	BENCH_BEGIN("cp_bbs_gen") {
		BENCH_ADD(cp_bbs_gen(d, p, z));
	}
	BENCH_END;

	BENCH_BEGIN("cp_bbs_sign") {
		BENCH_ADD(cp_bbs_sign(&b, s, msg, 5, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_bbs_ver") {
		BENCH_ADD(cp_bbs_ver(b, s, msg, 5, p, z));
	}
	BENCH_END;

	g1_free(s);
	bn_free(d);
	g2_free(p);
}
#endif

int main(void) {
	core_init();
	conf_print();

	util_print_banner("Benchmarks for the CP module:", 0);

#if defined(WITH_BN)
	util_print_banner("Protocols based on prime factorization:\n", 0);
	rsa();
	rabin();
#endif

#if defined(WITH_EC)
	util_print_banner("Protocols based on elliptic curves:\n", 0);
	if (ec_param_set_any() == STS_OK) {
		ecdh();
		ecdsa();
	} else {
		THROW(ERR_NO_CURVE);
	}
#endif

#if defined(WITH_PC)
	util_print_banner("Protocols based on pairings:\n", 0);
	if (pc_param_set_any() == STS_OK) {
		sokaka();
		bls();
		bbs();
	} else {
		THROW(ERR_NO_CURVE);
	}
#endif

	core_clean();
	return 0;
}
