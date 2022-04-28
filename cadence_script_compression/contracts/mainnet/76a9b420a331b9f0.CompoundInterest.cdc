// Compound interest, periodic compounding with 1 second period.
//
// This contract mostly reinvents the wheel -- waiting for https://github.com/onflow/cadence/issues/1151
// This all will be one-liner with exponentiaion that support fractions.
//
// Math here is based on the work of this guy and his articles. Kudos to him:
//   * https://medium.com/coinmonks/math-in-solidity-part-4-compound-interest-512d9e13041b
//   * https://medium.com/coinmonks/math-in-solidity-part-5-exponent-and-logarithm-9aef8515136e
//
// Math is adapted from base 2 to base 10. And taking into account Cadence UFix64 low precision.
//
// The formula is: 10^(t * log10(1+r)), where t is amount of periods and r is per-second interest ratio.
//
// We consider that one year has 31556952 seconds, and, for example, r for 3% APY is 0.000000000936681155.
// So that 10^(31556952 * log10(1+0.000000000936681155) = 1.0300000007092494.
// Here CompoundInterest.generatedCompoundInterest(seconds: 31556952.0, k: CompoundInterest.getK(3)) = 1.02999994
// Which is enough for practical application.
//
// The problem is that Cadence does not have neither exponentiation nor logarithm, and also lacks needed precision.
//   * base 10 exponentiation that support fractions is implemented here
//   * logarithms log10(1+r) are pre-computed and we denote it as k
//   * we use scaling factor of 10^7 to increase precision
pub contract CompoundInterest {

    access(self) let magic_factors: [[UFix64; 10]; 8]
    access(self) let powers_of_10: [UFix64; 12]
    access(self) let ks: [UFix64; 101]
    pub let k15: UFix64 // k for 15% APY
    pub let k100: UFix64 // k for 100% APY
    pub let k200: UFix64 // k for 200% APY
    pub let k2000: UFix64 // k for 2000% APY

    init() {

        // Magic factors can be generated using this Python script:
        //
        // import math
        // for i in range(1,9):
        //   print('[' + ', '.join(str("{:1.8f}".format(math.pow(10, 1/(10**i))**j)) for j in range(10)) + '],')
        self.magic_factors = [
            [1.00000000, 1.25892541, 1.58489319, 1.99526231, 2.51188643, 3.16227766, 3.98107171, 5.01187234, 6.30957344, 7.94328235],
            [1.00000000, 1.02329299, 1.04712855, 1.07151931, 1.09647820, 1.12201845, 1.14815362, 1.17489755, 1.20226443, 1.23026877],
            [1.00000000, 1.00230524, 1.00461579, 1.00693167, 1.00925289, 1.01157945, 1.01391139, 1.01624869, 1.01859139, 1.02093948],
            [1.00000000, 1.00023029, 1.00046062, 1.00069101, 1.00092146, 1.00115196, 1.00138251, 1.00161311, 1.00184377, 1.00207448],
            [1.00000000, 1.00002303, 1.00004605, 1.00006908, 1.00009211, 1.00011514, 1.00013816, 1.00016119, 1.00018422, 1.00020725],
            [1.00000000, 1.00000230, 1.00000461, 1.00000691, 1.00000921, 1.00001151, 1.00001382, 1.00001612, 1.00001842, 1.00002072],
            [1.00000000, 1.00000023, 1.00000046, 1.00000069, 1.00000092, 1.00000115, 1.00000138, 1.00000161, 1.00000184, 1.00000207],
            [1.00000000, 1.00000002, 1.00000005, 1.00000007, 1.00000009, 1.00000012, 1.00000014, 1.00000016, 1.00000018, 1.00000021]
        ]

        // UFix64 -- max is 184467440737.09551615, so max 11-th power of 10
        self.powers_of_10 = [
            1.0,
            10.0,
            100.0,
            1000.0,
            10000.0,
            100000.0,
            1000000.0,
            10000000.0,
            100000000.0,
            1000000000.0,
            10000000000.0,
            100000000000.0
        ]

        // For Starly we want APY to be 15%. So the numbers below are for that number.
        //
        // With given parameters if you put 1 $STARLY for 1 year (31556952 seconds), you would get 1.14999997 at the end.
        // Precision should be enough. Same math (using math.pow and math.log10) would give 1.150000001904556 in Python.
        //
        // Python snippets to get the numbers:
        // import math
        // interest = 1.15 # equivalent of annual percent
        // t = 31556952 # seconds in year
        // r = math.pow(interest, 1/t) - 1 # 4.428879707418787e-09 -- per-second interest ratio
        // k = math.log10(1+r) # 1.9234380136859298e-09
        //
        // Cadence UFix64 lacks precision, so we will do some scaling
        //
        // doing ks for 0-100%, calculated using this Python snippet:
        // import math
        // t = 31556952
        // for i in range(41):
        //   interest = 1 + i/100
        //   r = math.pow(interest, 1/t) - 1
        //   k = math.log10(1+r)
        //   k_scaled = k * 10**7 # multiple k by scaling factor 10^7
        //   print("{:1.8f},".format(k_scaled))
        self.ks = [
            0.00000000,
            0.00136939,
            0.00272529,
            0.00406795, // 3% APY
            0.00539765,
            0.00671462,
            0.00801911,
            0.00931135,
            0.01059157,
            0.01185998,
            0.01311682,
            0.01436228,
            0.01559657,
            0.01681989,
            0.01803243,
            0.01923438, // 15% APY
            0.02042592,
            0.02160724,
            0.02277850,
            0.02393988,
            0.02509154,
            0.02623364,
            0.02736634,
            0.02848980,
            0.02960415,
            0.03070956,
            0.03180616,
            0.03289409,
            0.03397349,
            0.03504448,
            0.03610721, // 30% APY
            0.03716179,
            0.03820836,
            0.03924702,
            0.04027791,
            0.04130113,
            0.04231680,
            0.04332502,
            0.04432592,
            0.04531959,
            0.04630613,
            0.04728565,
            0.04825825,
            0.04922403,
            0.05018308,
            0.05113548,
            0.05208135,
            0.05302075,
            0.05395379,
            0.05488054,
            0.05580110,
            0.05671554,
            0.05762394,
            0.05852638,
            0.05942295,
            0.06031371,
            0.06119875,
            0.06207813,
            0.06295192,
            0.06382021,
            0.06468305,
            0.06554051,
            0.06639266,
            0.06723957,
            0.06808131,
            0.06891792,
            0.06974948,
            0.07057604,
            0.07139767,
            0.07221442,
            0.07302636,
            0.07383353,
            0.07463599,
            0.07543381,
            0.07622702,
            0.07701569,
            0.07779987,
            0.07857960,
            0.07935494,
            0.08012594,
            0.08089264,
            0.08165509,
            0.08241334,
            0.08316744,
            0.08391743,
            0.08466335,
            0.08540525,
            0.08614318,
            0.08687716,
            0.08760726,
            0.08833350,
            0.08905593,
            0.08977459,
            0.09048951,
            0.09120074,
            0.09190831,
            0.09261226,
            0.09331263,
            0.09400946,
            0.09470277,
            0.09539261 // 100% APY
        ]

        self.k15 = self.ks[15]
        self.k100 = self.ks[100]
        self.k200 = 0.15119371
        self.k2000 = 0.41899461
    }

    pub fun getK(_ i: UInt8): UFix64 {
        return self.ks[i]
    }

    // Exponentiation with base 10
    pub fun pow10(_ x: UFix64): UFix64 {
        pre {
            // log10(184467440737.09551615) = 11.26591972249479
            x <= 11.26591972: "Cannot exceed Cadence UFix64 limit 184467440737.09551615"
        }
        // e.g for x = 3.14159265, we want:
        // * x_integer = 3
        // * f1 = 1
        // * f2 = 4
        // * f3 = 1
        // * f4 = 5
        // * etc
        // and multiply corresponding magic factors from the table
        let x_m1 = x % 1.0
        let x_m2 = x % 0.1
        let x_m3 = x % 0.01
        let x_m4 = x % 0.001
        let x_m5 = x % 0.0001
        let x_m6 = x % 0.00001
        let x_m7 = x % 0.000001
        let x_m8 = x % 0.0000001
        let x_integer = UInt8(x - x_m1);
        let f1 = UInt8((x_m1 - x_m2) * 10.0)
        let f2 = UInt8((x_m2 - x_m3) * 100.0)
        let f3 = UInt8((x_m3 - x_m4) * 1000.0)
        let f4 = UInt8((x_m4 - x_m5) * 10000.0)
        let f5 = UInt8((x_m5 - x_m6) * 100000.0)
        let f6 = UInt8((x_m6 - x_m7) * 1000000.0)
        let f7 = UInt8((x_m7 - x_m8) * 10000000.0)
        let f8 = UInt8(x_m8 * 100000000.0)
        return self.powers_of_10[x_integer]
            * self.magic_factors[0][f1]
            * self.magic_factors[1][f2]
            * self.magic_factors[2][f3]
            * self.magic_factors[3][f4]
            * self.magic_factors[4][f5]
            * self.magic_factors[5][f6]
            * self.magic_factors[6][f7]
            * self.magic_factors[7][f8]
    }

    // Get the generated compound interest for given number of seconds (periods) and k.
    // k = log10(1+r) * 10^7, where r is per-second interest ratio, e.g for 3% r = 0.000000000936681155,
    // since those numbers are very tiny for Cadence UFix64 precision, we use scaling factor of 10^7 to compensate it.
    // Use CompoundInterest.ks for precomputed values, e.k ks[1] = 1% APY, ks[8] = 8% APY.
    pub fun generatedCompoundInterest(seconds: UFix64, k: UFix64): UFix64 {
        // Applying same inverse scaling factor 10^7 for seconds, in the end those scaling factors will cancel out.
        let seconds_scaled = seconds / 10000000.0
        return self.pow10(seconds_scaled * k)
    }
}
