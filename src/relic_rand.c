/*
 * RELIC is an Efficient LIbrary for Cryptography
 * Copyright (C) 2007-2012 RELIC Authors
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
 * Implementation of the pseudo-random number generator.
 *
 * @version $Id$
 * @ingroup rand
 */

#include <stdlib.h>
#include <stdint.h>

#include "relic_conf.h"
#include "relic_core.h"
#include "relic_rand.h"
#include "relic_md.h"
#include "relic_error.h"

/*============================================================================*/
/* Private definitions                                                        */
/*============================================================================*/

#include <string.h>

#if SEED == DEV || SEED == UDEV

#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>

/**
 * The path to the char device that supplies entropy.
 */
#if SEED == DEV
#define RAND_PATH		"/dev/random"
#else
#define RAND_PATH		"/dev/urandom"
#endif

#elif SEED == WCGR

#include <windows.h>
#include <Wincrypt.h>
/* Avoid redefinition warning. */
#undef ERROR

#endif

/*============================================================================*/
/* Public definitions                                                         */
/*============================================================================*/

void rand_init() {
	unsigned char buf[MD_LEN_SHONE];

	memset(core_get()->rand, 0, RAND_SIZE);

#if SEED == ZERO

	memset(buf, 0, sizeof(buf));

#elif SEED == DEV || SEED == UDEV
	int rand_fd, c, l;

	rand_fd = open(RAND_PATH, O_RDONLY);
	if (rand_fd == -1) {
		THROW(ERR_NO_FILE);
	}

	l = 0;
	do {
		c = read(rand_fd, buf + l, MD_LEN_SHONE - l);
		l += c;
		if (c == -1) {
			THROW(ERR_NO_READ);
		}
	} while (l < MD_LEN_SHONE);

	if (rand_fd != -1) {
		close(rand_fd);
	}

#elif SEED == LIBC

#if OPSYS == FREEBSD
	srandom(1);
	for (int i = 0; i < MD_LEN_SHONE; i++) {
		buf[i] = (unsigned char)random();
	}
#else
	srand(1);
	for (int i = 0; i < MD_LEN_SHONE; i++) {
		buf[i] = (unsigned char)rand();
	}
#endif

#elif SEED == WCGR
	HCRYPTPROV   hCryptProv;

	if (!CryptAcquireContext(&hCryptProv, NULL, NULL, PROV_RSA_FULL, 0)) {
		THROW(ERR_NO_FILE);
	}
	if (hCryptProv && !CryptGenRandom(hCryptProv, MD_LEN_SHONE, buf)) {
		THROW(ERR_NO_READ);
	}
	if (hCryptProv && !CryptReleaseContext(hCryptProv, 0)) {
		THROW(ERR_NO_READ);
	}
#endif

	rand_seed(buf, MD_LEN_SHONE);
}

void rand_clean() {
	memset(core_get()->rand, 0, sizeof(core_get()->rand));
}

void rand_seed(unsigned char *buf, int size) {
    int i;
    ctx_t *ctx = core_get();

    if (size < MD_LEN_SHONE) {
    	THROW(ERR_NO_VALID);
    }

    /* Zero the current state. */
    memset(ctx->rand, 0, sizeof(ctx->rand));

    /* XKEY = SEED  */
    for (i = 0; i < MIN(size, MD_LEN_SHONE); i++) {
        ctx->rand[i] = buf[i];
    }
}

void rand_bytes(unsigned char *buf, int size) {
    unsigned char carry, c0, c1, r0, r1;
    int i, j;
    unsigned char hash[20];
    ctx_t *ctx = core_get();

    j = 0;
    while (j < size) {
        /* x = G(t, XKEY) */
        md_map_shone_mid(ctx->rand, 64, hash);

        /* XKEY = (XKEY + x + 1) mod 2^b */
        carry = 1;
        for (i = MD_LEN_SHONE - 1; i >= 0; i--) {
    		r0 = (unsigned char)(ctx->rand[i] + hash[i]);
    		c0 = (unsigned char)(r0 < hash[i] ? 1 : 0);
    		r1 = (unsigned char)(r0 + carry);
    		c1 = (unsigned char)(r1 < r0 ? 1 : 0);
    		carry = (unsigned char)(c0 | c1);
    		ctx->rand[i] = r1;
        }
        for (i = 0; i < MD_LEN_SHONE && j < size; i++) {
            buf[j] = hash[i];
            j++;
        }
    }
}
