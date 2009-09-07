/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007, 2008, 2009 RELIC Authors
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

static void rsa(void) {
	rsa_pub_t pub;
	rsa_prv_t prv;
	unsigned char in[1000], new[1000];
	unsigned char out[BN_BITS / 8 + 1];
	int in_len, out_len, new_len;

	bn_new(pub.e);
	bn_new(pub.n);
	bn_new(prv.d);
	bn_new(prv.dp);
	bn_new(prv.dq);
	bn_new(prv.p);
	bn_new(prv.q);
	bn_new(prv.qi);
	bn_new(prv.n);

	BENCH_ONCE("cp_rsa_gen", cp_rsa_gen(&pub, &prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_enc") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_enc(out, &out_len, in, in_len, &pub));
		cp_rsa_dec(new, &new_len, out, out_len, &prv);
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_dec") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_enc(out, &out_len, in, in_len, &pub);
		BENCH_ADD(cp_rsa_dec(new, &new_len, out, out_len, &prv));
	} BENCH_END;

#if CP_RSA == BASIC || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_basic", cp_rsa_gen_basic(&pub, &prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_enc (basic)") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_enc(out, &out_len, in, in_len, &pub));
		new_len = in_len;
		cp_rsa_dec(new, &new_len, out, out_len, &prv);
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_dec_basic") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_enc(out, &out_len, in, in_len, &pub);
		BENCH_ADD(cp_rsa_dec_basic(new, &new_len, out, out_len, &prv));
	} BENCH_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_quick", cp_rsa_gen_quick(&pub, &prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_enc (quick)") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_enc(out, &out_len, in, in_len, &pub));
		new_len = in_len;
		cp_rsa_dec_quick(new, &new_len, out, out_len, &prv);
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_dec_quick") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		new_len = in_len;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_enc(out, &out_len, in, in_len, &pub);
		BENCH_ADD(cp_rsa_dec_quick(new, &new_len, out, out_len, &prv));
	} BENCH_END;
#endif

	BENCH_ONCE("cp_rsa_gen", cp_rsa_gen(&pub, &prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sign") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_sign(out, &out_len, in, in_len, &prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_ver") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_sign(out, &out_len, in, in_len, &prv);
		BENCH_ADD(cp_rsa_ver(out, out_len, in, in_len, &pub));
	} BENCH_END;

#if CP_RSA == BASIC || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_basic", cp_rsa_gen_basic(&pub, &prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sign_basic") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_sign_basic(out, &out_len, in, in_len, &prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_ver (basic)") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_sign(out, &out_len, in, in_len, &prv);
		BENCH_ADD(cp_rsa_ver(out, out_len, in, in_len, &pub));
	} BENCH_END;
#endif

#if CP_RSA == QUICK || !defined(STRIP)
	BENCH_ONCE("cp_rsa_gen_quick", cp_rsa_gen_quick(&pub, &prv, BN_BITS));

	BENCH_BEGIN("cp_rsa_sign_quick") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		BENCH_ADD(cp_rsa_sign_quick(out, &out_len, in, in_len, &prv));
	} BENCH_END;

	BENCH_BEGIN("cp_rsa_ver (quick)") {
		bn_size_bin(&in_len, pub.n);
		in_len -= 11;
		out_len = BN_BITS / 8 + 1;
		rand_bytes(in, in_len);
		cp_rsa_sign(out, &out_len, in, in_len, &prv);
		BENCH_ADD(cp_rsa_ver(out, out_len, in, in_len, &pub));
	} BENCH_END;
#endif

}

#if defined(WITH_EB)

static void ecdsa(void) {
	unsigned char msg[5] = { 0, 1, 2, 3, 4 };
	bn_t r = NULL, s = NULL, d = NULL;
	eb_t p = NULL;

	bn_new(r);
	bn_new(s);
	bn_new(d);
	eb_new(p);

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

#if CP_ECDSA == BASIC || !defined(STRIP)
	BENCH_BEGIN("cp_ecdsa_gen") {
		BENCH_ADD(cp_ecdsa_gen(d, p));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdsa_sign_basic") {
		BENCH_ADD(cp_ecdsa_sign_basic(r, s, msg, 5, d));
	}
	BENCH_END;

	BENCH_BEGIN("cp_ecdsa_ver_basic") {
		BENCH_ADD(cp_ecdsa_ver_basic(r, s, msg, 5, p));
	}
	BENCH_END;
#endif

}

#endif

int main(void) {
	core_init();
	conf_print();

	util_print_banner("Benchmarks for the CP module:", 0);

#if defined(WITH_BN)
	util_print_banner("Protocols based on prime factorization:\n", 0);
	rsa();
#endif

#if defined(WITH_EB)
	util_print_banner("Protocols based on elliptic curves:\n", 0);
#if defined(EB_STAND) && defined(EB_ORDIN)
	if (eb_param_set_any_ordin() == STS_OK) {
		eb_param_print();
		ecdsa();
	}
#endif

#if defined(EB_STAND) && defined(EB_KBLTZ)
	if (eb_param_set_any_kbltz() == STS_OK) {
		eb_param_print();
		ecdsa();
	}
#endif
#endif

	core_clean();
	return 0;
}
