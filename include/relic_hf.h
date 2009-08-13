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
 * @defgroup hf Hash functions.
 */

/**
 * @file
 *
 * Interface of the hash functions module.
 *
 * @version $Id$
 * @ingroup hf
 */

#ifndef RELIC_HF_H
#define RELIC_HF_H

#include "relic_conf.h"
#include "relic_types.h"

/**
 * Maps a byte vector to a fixed-length byte vector using the chosen hash
 * function.
 *
 * @param[out] H				- the digest.
 * @param[in] M					- the message to hash.
 * @param[in] L					- the message length in bytes.
 */
#if HF_MAP == SHONE
#define hf_map(H, M, L)			hf_map_shone(H, M, L)
#elif HF_MAP == SH224
#define hf_map(H, M, L)			hf_map_sh224(H, M, L)
#elif HF_MAP == SH256
#define hf_map(H, M, L)			hf_map_sh256(H, M, L)
#elif HF_MAP == SH384
#define hf_map(H, M, L)			hf_map_sh384(H, M, L)
#elif HF_MAP == SH512
#define hf_map(H, M, L)			hf_map_sh512(H, M, L)
#endif

#define HF_LEN_SHONE			20

#define HF_LEN_SH224			28

#define HF_LEN_SH256			32

#define HF_LEN_SH384			48

#define HF_LEN_SH512			64

/**
 * Length in bytes of hash function output.
 */
#if HF_MAP == SHONE
#define HF_LEN					HF_LEN_SHONE
#elif HF_MAP == SH224
#define HF_LEN					HF_LEN_SH224
#elif HF_MAP == SH256
#define HF_LEN					HF_LEN_SH256
#elif HF_MAP == SH384
#define HF_LEN					HF_LEN_SH384
#elif HF_MAP == SH512
#define HF_LEN					HF_LEN_SH512
#endif

/**
 * Computes the SHA-1 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_shone(unsigned char *hash, unsigned char *msg, int len);

/**
 * Initializes the hash function context.
 */
void hf_map_shone_init(void);

/**
 * Updates the hash function context with more data.
 *
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_shone_update(unsigned char *msg, int len);

/**
 * Finalizes the hash function computation.
 *
 * @param[out] hash				- the digest.
 */
void hf_map_shone_final(unsigned char *hash);

/**
 * Returns the internal state of the hash function.
 *
 * @param[out] state			- the internal state.
 */
void hf_map_shone_state(unsigned char *state);

/**
 * Computes the SHA-224 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh224(unsigned char *hash, unsigned char *msg, int len);

/**
 * Computes the SHA-256 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh256(unsigned char *hash, unsigned char *msg, int len);

/**
 * Computes the SHA-384 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh384(unsigned char *hash, unsigned char *msg, int len);

/**
 * Computes the SHA-512 hash function.
 *
 * @param[out] hash				- the digest.
 * @param[in] msg				- the message to hash.
 * @param[in] len				- the message length in bytes.
 */
void hf_map_sh512(unsigned char *hash, unsigned char *msg, int len);

#endif /* !RELIC_HF_H */
