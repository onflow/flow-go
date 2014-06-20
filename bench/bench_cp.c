/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2014 RELIC Authors
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
 * Benchmarks for cryptographic protocols.
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
	uint8_t in[10], new[10], h[MD_LEN], out[BN_BITS / 8 + 1];
	int out_len, new_len;

	rsa_null(pub);
	rsa_null(prv);

	rsa_new(pub);
	rsa_new(prv);

	BENCH_ONCE("cp_rsa_gen", cp_rsa_gen(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_enc") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		BENCH_ADD(cp_rsa_enc(out, &out_len, in, sizeof(in), pub));
		cp_rsa_dec(new, &new_len, out, out_len, prv);
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_dec") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		cp_rsa_enc(out, &out_len, in, sizeof(in), pub);
		BENCH_ADD(cp_rsa_dec(new, &new_len, out, out_len, prv));
	} BENCH_END;

#if CP_RSA == BASIC || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_basic", cp_rsa_gen_basic(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_dec_basic") {
		out_len = BN_BITS / 8 + 1;
		new_len =out_len;
		rand_bytes(in, sizeof(in));
		cp_rsa_enc(out, &out_len, in, sizeof(in), pub);
		BENCH_ADD(cp_rsa_dec_basic(new, &new_len, out, out_len, prv));
	} BENCH_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_quick", cp_rsa_gen_quick(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_dec_quick") {
		out_len = BN_BITS / 8 + 1;
		new_len =out_len;
		rand_bytes(in, sizeof(in));
		cp_rsa_enc(out, &out_len, in, sizeof(in), pub);
		BENCH_ADD(cp_rsa_dec_quick(new, &new_len, out, out_len, prv));
	} BENCH_END;
#endif

	BENCH_ONCE("cp_rsa_gen", cp_rsa_gen(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sig (h = 0)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		BENCH_ADD(cp_rsa_sig(out, &out_len, in, sizeof(in), 0, prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_sig (h = 1)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		md_map(h, in, sizeof(in));
		BENCH_ADD(cp_rsa_sig(out, &out_len, h, MD_LEN, 1, prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_ver (h = 0)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		cp_rsa_sig(out, &out_len, in, sizeof(in), 0, prv);
		BENCH_ADD(cp_rsa_ver(out, out_len, in, sizeof(in), 0, pub));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_ver (h = 1)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		md_map(h, in, sizeof(in));
		cp_rsa_sig(out, &out_len, h, MD_LEN, 1, prv);
		BENCH_ADD(cp_rsa_ver(out, out_len, h, MD_LEN, 1, pub));
	} BENCH_END;

#if CP_RSA == BASIC || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_basic", cp_rsa_gen_basic(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sig_basic (h = 0)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		BENCH_ADD(cp_rsa_sig_basic(out, &out_len, in, sizeof(in), 0, prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_sig_basic (h = 1)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		md_map(h, in, sizeof(in));
		BENCH_ADD(cp_rsa_sig_basic(out, &out_len, h, MD_LEN, 1, prv));
	} BENCH_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_quick", cp_rsa_gen_quick(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sig_quick (h = 0)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		BENCH_ADD(cp_rsa_sig_quick(out, &out_len, in, sizeof(in), 0, prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_sig_quick (h = 1)") {
		out_len = BN_BITS / 8 + 1;
		new_len = out_len;
		rand_bytes(in, sizeof(in));
		md_map(h, in, sizeof(in));
		BENCH_ADD(cp_rsa_sig_quick(out, &out_len, in, sizeof(in), 1, prv));
	} BENCH_END;
#endif

	rsa_free(pub);
	rsa_free(prv);
}

static void rabin(void) {
	rabin_t pub, prv;
	uint8_t in[1000], new[1000], out[BN_BITS / 8 + 1];
	int in_len, out_len, new_len;

	rabin_null(pub);
	rabin_null(prv);

	rabin_new(pub);
	rabin_new(prv);

	BENCH_ONCE("cp_rabin_gen", cp_rabin_gen(pub, prv, BN_BITS));

	BENCH_BEGIN("cp_rabin_enc") {
		in_len = bn_size_bin(pub->n) - 9;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rabin_enc(out, &out_len, in, in_len, pub));
		cp_rabin_dec(new, &new_len, out, out_len, prv);
	} BENCH_END;

	BENCH_BEGIN("cp_rabin_dec") {
		in_len = bn_size_bin(pub->n) - 9;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rabin_enc(out, &out_len, in, in_len, pub);
		BENCH_ADD(cp_rabin_dec(new, &new_len, out, out_len, prv));
	} BENCH_END;

	rabin_free(pub);
	rabin_free(prv);
}

static void benaloh(void) {
	bdpe_t pub, prv;
	dig_t in, new;
	uint8_t out[BN_BITS / 8 + 1];
	int out_len;

	bdpe_null(pub);
	bdpe_null(prv);

	bdpe_new(pub);
	bdpe_new(prv);

	BENCH_ONCE("cp_bdpe_gen", cp_bdpe_gen(pub, prv, bn_get_prime(47), BN_BITS));

	BENCH_BEGIN("cp_bdpe_enc") {
		out_len = BN_BITS / 8 + 1;
		rand_bytes(out, 1);
		in = out[0] % bn_get_prime(47);
		BENCH_ADD(cp_bdpe_enc(out, &out_len, in, pub));
		cp_bdpe_dec(&new, out, out_len, prv);
	} BENCH_END;

	BENCH_BEGIN("cp_bdpe_dec") {
		out_len = BN_BITS / 8 + 1;
		rand_bytes(out, 1);
		in = out[0] % bn_get_prime(47);
		cp_bdpe_enc(out, &out_len, in, pub);
		BENCH_ADD(cp_bdpe_dec(&new, out, out_len, prv));
	} BENCH_END;

	bdpe_free(pub);
	bdpe_free(prv);
}

static void paillier(void) {
	bn_t n, l;
	uint8_t in[1000], new[1000], out[BN_BITS / 8 + 1];
	int in_len, out_len;

	bn_null(n);
	bn_null(l);

	bn_new(n);
	bn_new(l);

	BENCH_ONCE("cp_phpe_gen", cp_phpe_gen(n, l, BN_BITS / 2));

	BENCH_BEGIN("cp_phpe_enc") {
		in_len = bn_size_bin(n);
		out_len = BN_BITS / 8 + 1;
		memset(in, 0, sizeof(in));
		rand_bytes(in + 1, in_len - 1);
		BENCH_ADD(cp_phpe_enc(out, &out_len, in, in_len, n));
		cp_phpe_dec(new, in_len, out, out_len, n, l);
	} BENCH_END;

	BENCH_BEGIN("cp_bdpe_dec") {
		in_len = bn_size_bin(n);
		out_len = BN_BITS / 8 + 1;
		memset(in, 0, sizeof(in));
		rand_bytes(in + 1, in_len - 1);
		cp_phpe_enc(out, &out_len, in, in_len, n);
		BENCH_ADD(cp_phpe_dec(new, in_len, out, out_len, n, l));
	} BENCH_END;

	bn_free(n);
	bn_free(l);
}

#endif

#if defined(WITH_EC)

static void ecdh(void) {
	bn_t d;
	ec_t p;
	uint8_t key[MD_LEN];

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

static void ecmqv(void) {
	bn_t d1, d2;
	ec_t p1, p2;
	uint8_t key[MD_LEN];

	bn_null(d1);
	bn_null(d2);
	ec_null(p1);
	ec_null(p2);

	bn_new(d1);
	bn_new(d2);
	ec_new(p1);
	ec_new(p2);

	BENCH_BEGIN("cp_ecmqv_gen") {
		BENCH_ADD(cp_ecmqv_gen(d1, p1));
	}
	BENCH_END;

	cp_ecmqv_gen(d2, p2);

	BENCH_BEGIN("cp_ecmqv_key") {
		BENCH_ADD(cp_ecmqv_key(key, MD_LEN, d1, d2, p1, p1, p2));
	}
	BENCH_END;

	bn_free(d1);
	bn_free(d2);
	ec_free(p1);
	ec_free(p2);
}

static void ecies(void) {
	ec_t q, r;
	bn_t d;
	uint8_t in[10], out[16 + MD_LEN];
	int in_len, out_len;

	bn_null(d);
	ec_null(q);
	ec_null(r);

	ec_new(q);
	ec_new(r);
	bn_new(d);

	rand_bytes(in, sizeof(in));

	BENCH_BEGIN("cp_ecies_gen") {
		BENCH_ADD(cp_ecies_gen(d, q));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecies_enc") {
		in_len = sizeof(in);
		out_len = sizeof(out);
		rand_bytes(in, sizeof(in));
		BENCH_ADD(cp_ecies_enc(r, out, &out_len, in, in_len, q));
		cp_ecies_dec(out, &out_len, r, out, out_len, d);
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecies_dec") {
		in_len = sizeof(in);
		out_len = sizeof(out);
		rand_bytes(in, sizeof(in));
		cp_ecies_enc(r, out, &out_len, in, in_len, q);
		BENCH_ADD(cp_ecies_dec(out, &out_len, r, out, out_len, d));
	}
	BENCH_END;

	ec_free(q);
	ec_free(r);
	bn_free(d);
}

static void ecdsa(void) {
	uint8_t msg[5] = { 0, 1, 2, 3, 4 }, h[MD_LEN];
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

	BENCH_BEGIN("cp_ecdsa_sign (h = 0)") {
		BENCH_ADD(cp_ecdsa_sig(r, s, msg, 5, 0, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdsa_sign (h = 1)") {
		md_map(h, msg, 5);
		BENCH_ADD(cp_ecdsa_sig(r, s, h, MD_LEN, 1, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdsa_ver (h = 0)") {
		BENCH_ADD(cp_ecdsa_ver(r, s, msg, 5, 0, p));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdsa_ver (h = 1)") {
		md_map(h, msg, 5);
		BENCH_ADD(cp_ecdsa_ver(r, s, h, MD_LEN, 1, p));
	}
	BENCH_END;

	bn_free(r);
	bn_free(s);
	bn_free(d);
	ec_free(p);
}

static void ecss(void) {
	uint8_t msg[5] = { 0, 1, 2, 3, 4 };
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

	BENCH_BEGIN("cp_ecss_gen") {
		BENCH_ADD(cp_ecss_gen(d, p));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecss_sign") {
		BENCH_ADD(cp_ecss_sig(r, s, msg, 5, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecss_ver") {
		BENCH_ADD(cp_ecss_ver(r, s, msg, 5, p));
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
	sokaka_t k;
	bn_t s;
	uint8_t key1[MD_LEN];
	char id_a[5] = { 'A', 'l', 'i', 'c', 'e' };
	char id_b[3] = { 'B', 'o', 'b' };

	sokaka_null(k);

	sokaka_new(k);
	bn_new(s);

	cp_sokaka_gen(s);

	BENCH_BEGIN("cp_sokaka_gen") {
		BENCH_ADD(cp_sokaka_gen(s));
	}
	BENCH_END;

	BENCH_BEGIN("cp_sokaka_gen_prv") {
		BENCH_ADD(cp_sokaka_gen_prv(k, id_b, sizeof(id_b), s));
	}
	BENCH_END;

	BENCH_BEGIN("cp_sokaka_key (g1)") {
		BENCH_ADD(cp_sokaka_key(key1, MD_LEN, id_b, sizeof(id_b), k, id_a,
						sizeof(id_a)));
	}
	BENCH_END;

	if (pc_map_is_type3()) {
		cp_sokaka_gen_prv(k, id_a, sizeof(id_a), s);

		BENCH_BEGIN("cp_sokaka_key (g2)") {
			BENCH_ADD(cp_sokaka_key(key1, MD_LEN, id_a, sizeof(id_a), k, id_b,
							sizeof(id_b)));
		}
		BENCH_END;
	}

	sokaka_free(k);
	bn_free(s);
}

static void bls(void) {
	uint8_t msg[5] = { 0, 1, 2, 3, 4 };
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
		BENCH_ADD(cp_bls_sig(s, msg, 5, d));
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
	uint8_t msg[5] = { 0, 1, 2, 3, 4 }, h[MD_LEN];
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

	BENCH_BEGIN("cp_bbs_sign (h = 0)") {
		BENCH_ADD(cp_bbs_sig(s, msg, 5, 0, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_bbs_sign (h = 1)") {
		md_map(h, msg, 5);
		BENCH_ADD(cp_bbs_sig(s, h, MD_LEN, 1, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_bbs_ver (h = 0)") {
		BENCH_ADD(cp_bbs_ver(s, msg, 5, 0, p, z));
	}
	BENCH_END;

	BENCH_BEGIN("cp_bbs_ver (h = 1)") {
		md_map(h, msg, 5);
		BENCH_ADD(cp_bbs_ver(s, h, MD_LEN, 1, p, z));
	}
	BENCH_END;

	g1_free(s);
	bn_free(d);
	g2_free(p);
}
#endif

int main(void) {
	if (core_init() != STS_OK) {
		core_clean();
		return 1;
	}

	conf_print();

	util_banner("Benchmarks for the CP module:", 0);

#if defined(WITH_BN)
	util_banner("Protocols based on integer factorization:\n", 0);
	rsa();
	rabin();
	benaloh();
	paillier();
#endif

#if defined(WITH_EC)
	util_banner("Protocols based on elliptic curves:\n", 0);
	if (ec_param_set_any() == STS_OK) {
		ecdh();
		ecmqv();
		ecies();
		ecdsa();
		ecss();
	} else {
		THROW(ERR_NO_CURVE);
	}
#endif

#if defined(WITH_PC)
	util_banner("Protocols based on pairings:\n", 0);
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
