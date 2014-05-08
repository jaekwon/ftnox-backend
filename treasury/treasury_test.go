package treasury

import (
    //. "ftnox.com/common"
    "testing"
)

func TestSweepOutputs(t *testing.T) {

    validateOutput := func(total, minOutput, maxOutput uint64, maxNumOutputs int, outputs []uint64) {
        if len(outputs) > maxNumOutputs { t.Fatalf("number of outputs %v exceeds maxNumOutputs %v", len(outputs), maxNumOutputs) }
        sum := uint64(0)
        for _, output := range outputs {
            if output < minOutput { t.Fatalf("output %v is less than minOutput %v", output, minOutput) }
            if maxOutput < output { t.Fatalf("output %v is greater than maxOutput %v", output, maxOutput) }
            sum += output
        }
        if sum != total { t.Fatalf("sum of outputs didn't equal total: %v vs %v", sum, total) }
    }

    testValues := func(total, minOutput, maxOutput uint64, maxNumOutputs int) {
        outputAmounts, ok := computeSweepOutputs(total, minOutput, maxOutput, maxNumOutputs)
        if !ok {
            t.Fatalf("computeSweepOutputs failed")
        }
        validateOutput(total, minOutput, maxOutput, maxNumOutputs, outputAmounts)
    }

    testValues(uint64(1000),    uint64(10),     uint64(50),     100)
    testValues(uint64(1000),    uint64(10),     uint64(10),     100)
    testValues(uint64(1000),    uint64(10),     uint64(11),     100)
    testValues(uint64(5000),    uint64(10),     uint64(50),     100)

}
