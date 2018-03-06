import java.util.Arrays;
import java.util.Random;
import java.util.TreeMap;
import java.io.BufferedReader;
import java.io.File;
import java.io.FileReader;
import java.io.IOException;

public class KuhnTrainer {
    public static final int PASS = 0, BET = 1, NUM_ACTIONS = 2;
    public static final Random random = new Random(1);
    public TreeMap<String, Node> nodeMap = new TreeMap<String, Node>();

    static BufferedReader bufferedReader;
    static FileReader fileReader;
    static {
      File file = new File("rand.txt");
      try {
        fileReader = new FileReader(file);
      } catch (IOException e) {}
		  bufferedReader = new BufferedReader(fileReader);
    }

    class Node {
        String infoSet;
        double[] regretSum = new double[NUM_ACTIONS], 
                 strategy = new double[NUM_ACTIONS], 
                 strategySum = new double[NUM_ACTIONS];
        

        private double[] getStrategy(double realizationWeight) {
            double normalizingSum = 0;
            for (int a = 0; a < NUM_ACTIONS; a++) {
                strategy[a] = regretSum[a] > 0 ? regretSum[a] : 0;
                normalizingSum += strategy[a];
            }
            for (int a = 0; a < NUM_ACTIONS; a++) {
                if (normalizingSum > 0)
                    strategy[a] /= normalizingSum;
                else
                    strategy[a] = 1.0 / NUM_ACTIONS;
                strategySum[a] += realizationWeight * strategy[a];
            }
            return strategy;
        }
        

        public double[] getAverageStrategy() {
            double[] avgStrategy = new double[NUM_ACTIONS];
            double normalizingSum = 0;
            for (int a = 0; a < NUM_ACTIONS; a++)
                normalizingSum += strategySum[a];
            for (int a = 0; a < NUM_ACTIONS; a++) 
                if (normalizingSum > 0)
                    avgStrategy[a] = strategySum[a] / normalizingSum;
                else
                    avgStrategy[a] = 1.0 / NUM_ACTIONS;
            return avgStrategy;
        }
        

        public String toString() {
                return String.format("%4s: %s", infoSet, Arrays.toString(getAverageStrategy()));
        }

    }
    

    public void train(int iterations) {
        int[] cards = {1, 2, 3};
        double util = 0;
        for (int i = 0; i < iterations; i++) {
            for (int c1 = cards.length - 1; c1 > 0; c1--) { 
                int c2 = random.nextInt(c1 + 1);
                // System.out.println("c1c2: " + c1 + ", " + c2);
                int tmp = cards[c1];
                cards[c1] = cards[c2];
                cards[c2] = tmp;
            }
            String line = "";
            try {
              line = bufferedReader.readLine();
            } catch (IOException e) {}
            cards[0] = Integer.parseInt(line.substring(0, 1));
            cards[1] = Integer.parseInt(line.substring(1, 2));
            cards[2] = Integer.parseInt(line.substring(2, 3));
            // System.out.println("" + cards[0] + cards[1] + cards[2]);

            util += cfr(cards, "", 1, 1);
        }
        System.out.println("Average game value: " + util / iterations);
        for (Node n : nodeMap.values())
            System.out.println(n);
    }
    

    private double cfr(int[] cards, String history, double p0, double p1) {
        int plays = history.length();
        int player = plays % 2;
        int opponent = 1 - player;
        if (plays > 1) {
            boolean terminalPass = history.charAt(plays - 1) == 'p';
            boolean doubleBet = history.substring(plays - 2, plays).equals("bb");
            boolean isPlayerCardHigher = cards[player] > cards[opponent];
            if (terminalPass)
                if (history.equals("pp"))
                    return isPlayerCardHigher ? 1 : -1;
                else
                    return 1;
            else if (doubleBet)
                return isPlayerCardHigher ? 2 : -2;
        }               

        String infoSet = cards[player] + history;
        Node node = nodeMap.get(infoSet);
        if (node == null) {
            node = new Node();
            node.infoSet = infoSet;
            nodeMap.put(infoSet, node);
        }

        double[] strategy = node.getStrategy(player == 0 ? p0 : p1);
        double[] util = new double[NUM_ACTIONS];
        double nodeUtil = 0;
        for (int a = 0; a < NUM_ACTIONS; a++) {
            String nextHistory = history + (a == 0 ? "p" : "b");
            util[a] = player == 0 
                ? - cfr(cards, nextHistory, p0 * strategy[a], p1)
                : - cfr(cards, nextHistory, p0, p1 * strategy[a]);
            nodeUtil += strategy[a] * util[a];
        }

        for (int a = 0; a < NUM_ACTIONS; a++) {
            double regret = util[a] - nodeUtil;
            node.regretSum[a] += (player == 0 ? p1 : p0) * regret;
        }

        return nodeUtil;
    }
    

    public static void main(String[] args) {
        int iterations = 10000000;
        new KuhnTrainer().train(iterations);
    }

}
