/*
 * Copyright 2007 Project RELIC
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
 * @defgroup cp Cryptographic protocols.
 */

/**
 * @file
 *
 * Interface of the cryptographics protocols module.
 *
 * @version $Id$
 * @ingroup bn
 */

#ifndef RELIC_CP_H
#define RELIC_CP_H

#include <alloca.h>

#include "relic_conf.h"
#include "relic_types.h"
#include "relic_bn.h"
#include "relic_eb.h"
#include "relic_pb.h"

struct _rsa_pub_t {
	bn_t n;
	bn_t e;
};

typedef struct _rsa_pub_t rsa_pub_t;

struct _rsa_prv_t {
	bn_t d;
	bn_t p;
	bn_t q;
	bn_t dp;
	bn_t dq;
	bn_t qi;
	bn_t n;
};

typedef struct _rsa_prv_t rsa_prv_t;

#if CP_RSA == BASIC
#define cp_rsa_gen(PB, PV, B)				cp_rsa_gen_basic(PB, PV, B)
#elif CP_RSA == QUICK
#define cp_rsa_gen(PB, PV, B)				cp_rsa_gen_quick(PB, PV, B)
#endif

int cp_rsa_gen_basic(rsa_pub_t * pub, rsa_prv_t * prv, int bits);

int cp_rsa_gen_quick(rsa_pub_t * pub, rsa_prv_t * prv, int bits);

int cp_rsa_enc(unsigned char *out, int *out_len, unsigned char *in, int in_len,
		rsa_pub_t * pub);

#if CP_RSA == BASIC
#define cp_rsa_dec(O, OL, I, IL, P)			cp_rsa_dec_basic(O, OL, I, IL, P)
#elif CP_RSA == QUICK
#define cp_rsa_dec(O, OL, I, IL, P)			cp_rsa_dec_quick(O, OL, I, IL, P)
#endif

int cp_rsa_dec_basic(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, rsa_prv_t * prv);

int cp_rsa_dec_quick(unsigned char *out, int *out_len, unsigned char *in,
		int in_len, rsa_prv_t * prv);

#if CP_RSA == BASIC
#define cp_rsa_sign(O, OL, I, IL, P)		cp_rsa_sign_basic(O, OL, I, IL, P)
#elif CP_RSA == QUICK
#define cp_rsa_sign(O, OL, I, IL, P)		cp_rsa_sign_quick(O, OL, I, IL, P)
#endif

int cp_rsa_sign_basic(unsigned char *sig, int *sig_len, unsigned char *msg,
		int msg_len, rsa_prv_t * prv);

int cp_rsa_sign_quick(unsigned char *sig, int *sig_len, unsigned char *msg,
		int msg_len, rsa_prv_t * prv);

int cp_rsa_ver(unsigned char *sig, int sig_len, unsigned char *msg, int msg_len,
		rsa_pub_t * pub);

void cp_ecdsa_init(void);

void cp_ecdsa_gen(bn_t d, eb_t q);

#if CP_ECDSA == BASIC
#define cp_ecdsa_sign(R, S, M, L, D)	cp_ecdsa_sign_basic(R, S, M, L, D)
#define cp_ecdsa_ver(R, S, M, L, D)		cp_ecdsa_ver_basic(R, S, M, L, D)
#elif CP_ECDSA == QUICK
#define cp_ecdsa_sign(R, S, M, L, D)	cp_ecdsa_sign_quick(R, S, M, L, D)
#define cp_ecdsa_ver(R, S, M, L, D)		cp_ecdsa_ver_quick(R, S, M, L, D)
#endif

void cp_ecdsa_sign_basic(bn_t r, bn_t s, unsigned char *msg, int len, bn_t d);

void cp_ecdsa_sign_quick(bn_t r, bn_t s, unsigned char *msg, int len, bn_t d);

int cp_ecdsa_ver_basic(bn_t r, bn_t s, unsigned char *msg, int len, eb_t q);

int cp_ecdsa_ver_quick(bn_t r, bn_t s, unsigned char *msg, int len, eb_t q);

void cp_sokaka_gen(bn_t master);

void cp_sokaka_gen_pub(eb_t p, char *id, int len);

void cp_sokaka_gen_prv(eb_t s, char *id, int len, bn_t master);

void cp_sokaka_key(fb4_t key, eb_t p, eb_t s);

#endif /* !RELIC_CP_H */
